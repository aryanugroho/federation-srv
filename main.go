package main

import (
	"log"
	"strings"

	"github.com/micro/cli"
	"github.com/micro/go-micro"

	"github.com/micro/federation-srv/federation"
	"github.com/micro/federation-srv/handler"

	proto "github.com/micro/federation-srv/proto/federation"
	"github.com/micro/go-platform/config"
	"github.com/micro/go-platform/config/source/file"
)

var (
	defaultFile = "federation.json"
)

func main() {

	service := micro.NewService(
		micro.Name("go.micro.srv.federation"),
		micro.Flags(
			cli.StringFlag{
				Name:   "config_source",
				EnvVar: "CONFIG_SOURCE",
				Usage:  "Source to read the config from e.g file, platform",
			},
		),
	)

	// initialise service
	service.Init(
		micro.Action(func(c *cli.Context) {
			var src string

			parts := strings.Split(c.String("config_source"), ":")

			if len(parts) > 0 {
				src = parts[0]
			}

			var source config.Source

			switch src {
			case "platform":
				log.Println("Using platform source")
				source = config.NewSource(config.SourceClient(service.Client()))
			case "file":
				fileName := defaultFile

				if len(parts) > 1 {
					fileName = parts[1]
				}

				log.Println("Using file source:", fileName)
				source = file.NewSource(config.SourceName(fileName))
			default:
				fileName := defaultFile

				if len(parts) > 1 {
					fileName = parts[1]
				}

				log.Println("Using file source:", fileName)
				source = file.NewSource(config.SourceName(fileName))
			}

			federation.Init(config.NewConfig(config.WithSource(source)), service)
		}),
		micro.BeforeStart(federation.Run),
	)

	proto.RegisterFederationHandler(service.Server(), new(handler.Federation))

	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}
