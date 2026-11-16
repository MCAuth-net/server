FROM golang:1.27-alpine AS build

WORKDIR /app
COPY . .
RUN touch server-icon.png
RUN go build -o server .

FROM alpine:3
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /app/server server
COPY --from=build /app/server-icon.png server-icon.png
CMD ["./server"]