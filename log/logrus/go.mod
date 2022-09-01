module github.com/dobyte/due/log/logrus

go 1.16

require (
	github.com/dobyte/due v0.0.1
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/lestrrat-go/strftime v1.0.6 // indirect
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sirupsen/logrus v1.9.0
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
)

replace github.com/dobyte/due => ./../../
