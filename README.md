# Federation Service

The federation service is a mechanism for federating messages globally agnostic of the underlying architecture or topology. 
Federation relies on the pluggable go-micro broker which means the message bus can be swapped out without changing the code.

## Prerequisites

The federation service depends on a config source which can be set with `--config_source`. It current accepts 'file' or 'platform' 
and defaults to the file source. When using file source the default file is federation.json in the current directory or 
you can specify a file like so `--config_source=file:/path/to/file.json`.

## Config

The federation service is fairly straight forward in that it subscribes to topics in one environment and publishes to 
topics in another with the name `[topic].federated`. The main work is the configuration itself. Config is keyed 
under 'federation' with sections for 'topics' and  'brokers'. Topics specify the topics to be federated, at what sample rate 
and which regions to subscribe/publish. Brokers specifies the brokers for each region and is broken down by plugin type. 

Example config:

```json
{
	"federation": {
		"topics": {
			"events": {
				"rate": 1.0,
				"subscribe": [
					"eu-west-1"

				],
				"publish": [
					"us-east-1",
					"private-dc"
				]
			},
			"messages": {
				"rate": 0.1,
				"subscribe": [
					"us-east-1"

				],
				"publish": [
					"eu-west-1"
				]
			}
		},
		"brokers": {
			"eu-west-1": {
				"nsq": {
					"hosts": ["127.0.0.1:4150"]
				},
				"nats": {
					"hosts": ["127.0.0.1:4222"]
				}
			},
			"us-east-1": {
				"rabbitmq": {
					"hosts": ["127.0.0.1:5672"]
				}
			},
			"private-dc": {
				"kafka": {
					"hosts": ["192.168.99.100:9092"]
				}
			}
		}
	}
}
```

## Getting started

1. Install Consul

	Consul is the default registry/discovery for go-micro apps. It's however pluggable.
	[https://www.consul.io/intro/getting-started/install.html](https://www.consul.io/intro/getting-started/install.html)

2. Run Consul
	```
	$ consul agent -dev -advertise=127.0.0.1
	```

3. Download and start the service

	```shell
	go get github.com/micro/federation-srv
	federation-srv
	```

	OR as a docker container

	```shell
	docker run microhq/federation-srv --registry_address=YOUR_REGISTRY_ADDRESS
	```

## API

Federation
- Config

### Federation.Config

```shell
micro query go.micro.srv.federation  Federation.Config
{
	"config": {
		"brokers": {
			"eu-west-1": {
				"plugins": {
					"nats": {
						"hosts": [
							"127.0.0.1:4222"
						]
					},
					"nsq": {
						"hosts": [
							"127.0.0.1:4150"
						]
					}
				}
			},
			"private-dc": {
				"plugins": {
					"kafka": {
						"hosts": [
							"192.168.99.100:9092"
						]
					}
				}
			},
			"us-east-1": {
				"plugins": {
					"rabbitmq": {
						"hosts": [
							"127.0.0.1:5672"
						]
					}
				}
			}
		},
		"topics": {
			"events": {
				"publish": [
					"us-east-1",
					"private-dc"
				],
				"rate": 1,
				"subscribe": [
					"eu-west-1"
				]
			},
			"messages": {
				"publish": [
					"eu-west-1"
				],
				"rate": 0.1,
				"subscribe": [
					"us-east-1"
				]
			}
		}
	}
}
```
