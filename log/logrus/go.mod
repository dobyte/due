module github.com/dobyte/due/log/logrus

go 1.16

require (
    github.com/sirupsen/logrus v1.9.0 // indirect
    github.com/dobyte/due v0.0.1
)

replace (
	github.com/dobyte/due => ./../../
)