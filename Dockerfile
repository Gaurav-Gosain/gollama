FROM golang:1.22 as ollama-build

WORKDIR /go/src/app
COPY . .

RUN go mod download &&\
  CGO_ENABLED=0 go build -o /go/bin/ollama

# RUN go vet -v
# RUN go test -v


FROM gcr.io/distroless/static-debian11:nonroot

ENV TERM=xterm-256color
COPY --from=ollama-build /go/bin/ollama /
ENTRYPOINT ["/ollama"]


