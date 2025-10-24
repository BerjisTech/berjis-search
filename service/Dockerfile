FROM golang:1.22 as build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/search-service ./cmd/service && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/search-crawler ./cmd/crawler && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/search-scheduler ./cmd/scheduler

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=build /out/search-service /app/search-service
COPY --from=build /out/search-crawler /app/search-crawler
COPY --from=build /out/search-scheduler /app/search-scheduler
ENV PORT=8092
USER nonroot
EXPOSE 8092
ENTRYPOINT ["/app/search-service"]
