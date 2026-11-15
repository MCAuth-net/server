FROM golang:1.27-alpine AS build

WORKDIR /app
COPY . .
RUN go build -o server .

FROM alpine:3
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /app/server server
CMD ["./server"]