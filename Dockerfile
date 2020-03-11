FROM golang:1.12 as builder
WORKDIR /go/src/github.com/rcarrillocruz/egress-router
COPY . .
RUN go build -o /go/bin/egress-router ./plugin/main.go

FROM openshift/origin-base

COPY --from=builder /go/bin/egress-router /egress-router
