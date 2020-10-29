FROM golang as builder

ENV GO111MODULE=on

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build

FROM scratch

VOLUME /labels

COPY --from=builder /app/k8s-auto-labeller /app/
COPY --from=builder /app/labels/ /labels/

ENTRYPOINT [ "/app/k8s-auto-labeller", "-label-dir", "/labels" ]
