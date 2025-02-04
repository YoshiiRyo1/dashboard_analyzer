FROM golang:1.22.3 as build
WORKDIR /app
COPY go.mod go.sum ./
COPY *.go ./
RUN go build -tags lambda.norpc -ldflags="-s -w" -o main .

# Copy artifacts to a clean image
FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build /app/main ./bootstrap

ENTRYPOINT [ "./bootstrap" ]
