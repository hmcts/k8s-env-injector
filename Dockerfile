#FROM alpine:latest
FROM gcr.io/distroless/static

ADD k8s-env-injector /k8s-env-injector
ENTRYPOINT ["./k8s-env-injector"]
