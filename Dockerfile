# Start with a base Go image for compilation
FROM --platform=$BUILDPLATFORM golang:1.21.3-bullseye AS builder

RUN apt update -y && apt install upx -y

# trivy installation - but i comment it ... no plan to use it
# RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/master/contrib/install.sh | sh -s -- -b /usr/local/bin 

WORKDIR /app

ARG TARGETARCH

COPY go.mod ./

COPY go.sum ./

RUN go mod download

COPY . .

# Build the Go binary
# RUN go build -o cdddru
# RUN go build -ldflags="-s -w" -o myapp
RUN GOOS=linux GOARCH=$TARGETARCH CGO_ENABLED=0 go build -ldflags="-s -w" -o cdddru
# go build -ldflags="-s -w" -trimpath -o myapp

RUN upx cdddru

# Start a new image to keep it lightweight
FROM docker:24.0.6-git

# Install necessary dependencies
RUN apk update
RUN apk --no-cache add ca-certificates curl git

ARG TARGETARCH

RUN if [ "$TARGETARCH" = "amd64" ]; then \
        curl -LO "https://golang.org/dl/go1.21.3.linux-amd64.tar.gz" && \
        tar -C /usr/local -xzf go1.21.3.linux-amd64.tar.gz && \
        rm go1.21.3.linux-amd64.tar.gz; \
    elif [ "$TARGETARCH" = "arm64" ]; then \
        curl -LO "https://golang.org/dl/go1.21.3.linux-arm64.tar.gz" && \
        tar -C /usr/local -xzf go1.21.3.linux-arm64.tar.gz && \
        rm go1.21.3.linux-arm64.tar.gz; \
    # Add more elif statements for other architectures as needed
    fi

# Set Go environment variables
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:$GOROOT/bin:$PATH

RUN mkdir -p /root/.vscode-server/extensions \
    && curl -L https://github.com/golang/vscode-go/releases/download/v0.39.1/go-0.39.1.vsix -o /tmp/go.vsix \
    && unzip /tmp/go.vsix -d /root/.vscode-server/extensions/ms-vscode.go \
    && rm /tmp/go.vsix \
    && go get golang.org/x/tools/gopls

# COPY trivy 
# COPY --from=builder /usr/local/bin/trivy /usr/local/bin/trivy

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
COPY ./jobs/ /app/jobs/
COPY ./manifests /app/manifests/
# Set the entrypoint to run the Go application by default
# ENTRYPOINT ["cdddru"]
CMD ["./cdddru", "-f", "./jobs/config.json"]

