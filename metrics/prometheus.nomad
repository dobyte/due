job "prometheus" {
  datacenters = ["dc1"]
  type        = "service"

  group "monitoring" {
    count = 1

    network {
      port "prometheus_ui" {
        static=9090
      }
    }

    restart {
      attempts = 2
      interval = "30m"
      delay    = "15s"
      mode     = "fail"
    }

    ephemeral_disk {
      size = 300
    }

    task "prometheus" {
      template {
        change_mode = "noop"
        destination = "local/prometheus.yml"

        data = <<EOH
scrape_configs:
- job_name: myapp
  consul_sd_configs:
# int
  - server: 'http://nlb-3jk6qjfyh8sf52gnzc.cn-beijing.nlb.aliyuncs.com:8500'
# prod
#  - server: 'http://nlb-q6yon5q2p71y8up7z9.cn-beijing.nlb.aliyuncs.com:8500'
    services: ['gate-exporter', 'node-exporter']

  scrape_interval: 30s
  params:
    format: ['prometheus']
remote_write:
# int
  - url: "http://10.3.56.78:8081/mi-metrics"
# prod
#  - url: "http://172.16.54.205:8081/mi-metrics"
    # Configures the queue used to write to remote storage.
    queue_config:
      # Number of samples to buffer per shard before we start dropping them.
      capacity: 10000
      # Maximum number of shards, i.e. amount of concurrency.
      max_shards: 1
      # Maximum number of samples per send.
      max_samples_per_send: 500
EOH
      }

      driver = "docker"

      config {
        image = "prom/prometheus:latest"

        volumes = [
          "local/prometheus.yml:/etc/prometheus/prometheus.yml",
        ]

        ports = ["prometheus_ui"]
      }

      resources {
        cpu    = 512
        memory = 1024
      }

      service {
        name = "prometheus"
        provider = "nomad"
        port = "prometheus_ui"
      }
    }
  }
}

#---
#scrape_configs:
#- job_name: myapp
#scrape_interval: 30s
#static_configs:
#- targets:
#- 10.1.122.162:8665
#- 10.2.10.64:8665
#- 10.1.122.132:8664
#- 10.2.10.61:8664
#- 10.1.122.117:25438
#remote_write:
#  - url: "http://10.1.122.117:26036/mi-metrics"
#    # Configures the queue used to write to remote storage.
#    queue_config:
#      # Number of samples to buffer per shard before we start dropping them.
#      capacity: 10000
#      # Maximum number of shards, i.e. amount of concurrency.
#      max_shards: 1
#      # Maximum number of samples per send.
#      max_samples_per_send: 500