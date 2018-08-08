FROM hub.ibitcome.com/library/golang:1.10-dgateway
WORKDIR /go
ADD . /go/src/dgateway
RUN mkdir -p /data/bitmain/leveldb_data ;\
    cp -a /tmp/vendor/github.com/golang /go/src/github.com/ ;\
    cp -a /tmp/vendor/github.com/julienschmidt/ /go/src/github.com/ ;\
    cp -a /tmp/vendor/github.com/rcrowley/ /go/src/github.com/ ;\
    cp -a /tmp/vendor/github.com/influxdata /go/src/github.com/ ;\
    cp -a /tmp/vendor/github.com/rs/ /go/src/github.com/ ;\
    cp -a /tmp/vendor/github.com/vrischmann/ /go/src/github.com/ ;\
    cp -a /tmp/vendor/golang.org/x/net/ /go/src/golang.org/x ;\
    cp -a /tmp/vendor/google.golang.org/ /go/src/ ;\
    cp -a /tmp/vendor/gopkg.in/ /go/src/ ;\
#    cp -a /tmp/vendor /go/src/dgateway ;\
    cd /go/src/dgateway/braftd ;\
    go install
CMD ["braftd","--config","/go/src/dgateway/config.toml"]
