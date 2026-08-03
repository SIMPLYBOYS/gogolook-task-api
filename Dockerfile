FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /task-api

FROM gcr.io/distroless/static-debian12
COPY --from=build /task-api /task-api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/task-api"]
