FROM golang:1.24.6-bullseye as build

WORKDIR /go/src/app
ADD . /go/src/app
RUN go get -u -t ./...
RUN CGO_ENABLED=0 GOOS=linux GO111MODULE="on" go build -a -installsuffix cgo -o /go/bin/app/k8s-env-injector .


FROM gcr.io/distroless/static

COPY --from=build /go/bin/app /
ENTRYPOINT ["./k8s-env-injector"]
