FROM golang:1.25-alpine
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/usersvc ./cmd/usersvc
RUN go build -o /usersvc ./cmd/usersvc
EXPOSE 8090
CMD ["/usersvc"]
