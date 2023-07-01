# Start with a base Go image for compilation
FROM golang:1.20 AS builder

RUN apt update -y && apt install upx -y
# Set the working directory inside the container
WORKDIR /app

# Copy the Go source code to the container
COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

# Build the Go binary
# RUN go build -o myapp
# RUN go build -ldflags="-s -w" -o myapp
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o cdddru
# go build -ldflags="-s -w" -trimpath -o myapp
RUN upx cdddru

# Start a new image to keep it lightweight
FROM docker:20.10.24-cli-alpine3.18

# Install necessary dependencies
RUN apk --no-cache add ca-certificates curl git

# Install kubectl and kubectx
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && mv kubectl /usr/local/bin/

RUN git clone https://github.com/ahmetb/kubectx /opt/kubectx && \
    ln -s /opt/kubectx/kubectx /usr/local/bin/kubectx && \
    ln -s /opt/kubectx/kubens /usr/local/bin/kubens

# Install popular network debugging tools
RUN apk --no-cache add tcpdump netcat-openbsd bind-tools openssh bash rsync

RUN mkdir -p ~/.ssh && ssh-keyscan github.com >> ~/.ssh/known_hosts

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/cdddru /app/
COPY ./tasks/ /app/tasks/
COPY ./manifests /app/manifests/
# Set the entrypoint to run the Go application by default
# ENTRYPOINT ["cdddru"]
CMD ["cdddru ./tasks/config.json"]
