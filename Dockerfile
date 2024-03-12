FROM golang:1.22 as gollama-build

WORKDIR /go/src/app
COPY . .

RUN go mod download &&\
  CGO_ENABLED=0 go build -o /go/bin/gollama

# RUN go vet -v
# RUN go test -v


FROM gcr.io/distroless/static-debian11:nonroot

ENV TERM=xterm-256color
COPY --from=gollama-build /go/bin/gollama /
ENTRYPOINT ["/gollama"]


