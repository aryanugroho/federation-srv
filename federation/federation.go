package federation

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/micro/go-micro"
	"github.com/micro/go-micro/broker"
	"github.com/micro/go-os/config"

	// the plugins we know about
	"github.com/micro/go-micro/broker/http"
	"github.com/micro/go-plugins/broker/kafka"
	"github.com/micro/go-plugins/broker/nats"
	"github.com/micro/go-plugins/broker/nsq"
	"github.com/micro/go-plugins/broker/rabbitmq"
)

// Config is the structure we expect for Federation Config
type Config struct {
	// name:config
	Topics map[string]Topic `json:"topics"`
	// region:config
	Brokers map[string]Broker `json:"brokers"`
}

type Topic struct {
	// rate at which to publish
	Rate float64 `json:"rate"`
	// list of publisher regions
	Publish []string `json:"publish"`
	// list of subscriber regions
	Subscribe []string `json:"subscribe"`
}

// Broker name:config
type Broker map[string]Plugin

type Plugin struct {
	Hosts []string `json:"hosts"`
}

type federator struct {
	exit    chan bool
	config  config.Config
	service micro.Service

	sync.Mutex
	running bool
	c       Config
}

var (
	// default instance of the federator
	defaultFederator = newFederator()

	// federated topic extension
	federateExt = ".federated"

	// Broker plugins supported
	Plugins = map[string]func(...broker.Option) broker.Broker{
		"http":     http.NewBroker,
		"kafka":    kafka.NewBroker,
		"nats":     nats.NewBroker,
		"nsq":      nsq.NewBroker,
		"rabbitmq": rabbitmq.NewBroker,
	}
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func newFederator() *federator {
	return &federator{}
}

func (f *federator) getConfig() Config {
	f.Lock()
	c := f.c
	f.Unlock()
	return c
}

func (f *federator) init(c config.Config, s micro.Service) {
	f.config = c
	f.service = s
}

func (f *federator) extract() (Config, error) {
	var config Config

	if err := f.config.Get("federation").Scan(&config); err != nil {
		return config, errors.New("error reading config " + err.Error())
	}

	if len(config.Topics) == 0 {
		return config, errors.New("No topics found to federate")
	}

	if len(config.Brokers) == 0 {
		return config, errors.New("No brokers found to federate")
	}

	// TODO: validate config so we have no loops

	return config, nil
}

func (f *federator) federate() {
	// get config
	f.Lock()
	config := f.c
	f.Unlock()

	// region:brokers
	regions := make(map[string][]broker.Broker)

	// create regional brokers
	for region, brokers := range config.Brokers {
		log.Printf("[region:%s] Starting setup\n", region)

		// create publishers
		for name, plugin := range brokers {
			b, err := f.setup(name, plugin.Hosts)
			if err != nil {
				log.Println(err)
				continue
			}
			log.Printf("[region:%s] Adding broker %s %v\n", region, name, plugin.Hosts)
			regions[region] = append(regions[region], b)
		}
	}

	// teardown brokers
	teardown := func() {
		<-f.exit
		for region, brokers := range regions {
			log.Printf("[region:%s] Teardown\n", region)
			for _, broker := range brokers {
				broker.Disconnect()
			}
		}
	}

	go teardown()

	// create subscribers
	for topic, tConfig := range config.Topics {
		var pubs []broker.Broker
		var subs []broker.Broker

		// make sure there's a rate
		if tConfig.Rate <= 0.0 {
			log.Printf("[topic:%s][rate:%v] Rate too low to subscribe\n", topic, tConfig.Rate)
			continue
		}

		log.Printf("[topic:%s][rate:%v] Subscribing\n", topic, tConfig.Rate)

		// create subscriber list
		for _, sub := range tConfig.Subscribe {
			subs = append(subs, regions[sub]...)
		}

		// create publisher list
		for _, pub := range tConfig.Publish {
			pubs = append(pubs, regions[pub]...)
		}

		for _, sub := range subs {
			if err := f.sub(topic, tConfig.Rate, sub, pubs); err != nil {
				log.Printf("[topic:%s][plugin:%s][addr:%s] Failed to subscribe: %v\n", topic, sub.String(), sub.Address(), err.Error())
			}
		}
	}
}

func (f *federator) update() {
	log.Println("Updating federator config")

	// extract the config first
	c, err := f.extract()
	if err != nil {
		log.Println(err)
		return
	}

	// stop the federator
	f.stop()

	// update the config
	f.Lock()
	f.c = c
	f.Unlock()

	// start the federator
	f.start()
}

// run starts the config watcher and triggers an update on every change
func (f *federator) run(w config.Watcher, c Config) {
	var retries int

	// set the config
	f.Lock()
	f.c = c
	f.Unlock()

	// start the federator
	f.start()

	for {
		log.Println("Waiting for next federator config change")
		// wait for next config change
		_, err := w.Next()
		if err != nil {
			log.Println(err)
			if retries > 3 {
				// TODO: fix this
				log.Println("Watcher is dead... exiting")
				os.Exit(1)
			}
			retries++
			time.Sleep(time.Second)
			continue
		}

		// update the federator
		f.update()
		// reset retries
		retries = 0
	}
}

func (f *federator) setup(name string, hosts []string) (broker.Broker, error) {
	fn, ok := Plugins[name]
	if !ok {
		return nil, fmt.Errorf("unknown plugin %s", name)
	}

	b := fn(broker.Addrs(hosts...))

	if err := b.Init(); err != nil {
		return nil, fmt.Errorf("error initialising %s: %v", b.String(), err)
	}

	if err := b.Connect(); err != nil {
		return nil, fmt.Errorf("error connecting to %s: %v", b.String(), err)
	}

	return b, nil
}

// sub subscribers to a topic until exit
func (f *federator) sub(topic string, rate float64, sub broker.Broker, pubs []broker.Broker) error {
	if strings.HasSuffix(topic, federateExt) {
		return fmt.Errorf("Cannot federate %s has federate extension", topic)
	}

	// set pub topic
	pubTopic := topic + federateExt

	// TODO: subscription concurrency
	s, err := sub.Subscribe(topic, func(pb broker.Publication) error {
		// only publish at the rate specified
		if rate < rand.Float64() {
			return nil
		}

		// We don't do anything with the message
		// Just publish to all the publishers
		for _, pub := range pubs {
			if err := pub.Publish(pubTopic, pb.Message()); err != nil {
				log.Println("Error publishing to", pub.String(), "for topic", topic)
			}
		}

		return nil
	}, broker.Queue("federation"))

	// TODO: more resilient behaviou
	if err != nil {
		return err
	}

	// unsub on exit broadcast
	go func() {
		<-f.exit
		s.Unsubscribe()
	}()

	return nil
}

func (f *federator) start() {
	f.Lock()
	if f.running {
		f.Unlock()
		return
	}

	log.Println("Starting federator")

	f.exit = make(chan bool)
	f.running = true
	f.Unlock()
	f.federate()
}

func (f *federator) stop() {
	f.Lock()
	defer f.Unlock()

	if !f.running {
		return
	}

	log.Println("Stopping federator")
	f.running = false
	close(f.exit)
}

func GetConfig() Config {
	return defaultFederator.getConfig()
}

// Init initialises the config and service
func Init(c config.Config, s micro.Service) {
	defaultFederator.init(c, s)
}

// Run starts the federator
func Run() error {
	// watch path starting at federation
	log.Println("Watching path", "federation")

	w, err := defaultFederator.config.Watch("federation")
	if err != nil {
		return err
	}

	// extract the config first
	c, err := defaultFederator.extract()
	if err != nil {
		return err
	}

	// start the federator
	go defaultFederator.run(w, c)
	return nil
}
