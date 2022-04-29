# Build Stage
FROM --platform=linux/amd64 ubuntu:20.04 as builder

## Install build dependencies.
RUN apt-get update 
RUN DEBIAN_FRONTEND=noninteractive apt-get install -y wget
RUN apt-get install -y make
RUN apt-get install -y git
RUN apt-get install -y tar
RUN wget https://go.dev/dl/go1.18.1.linux-amd64.tar.gz
RUN tar -C /usr/local -xvf go1.18.1.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

## Add source code to the build stage.
ADD . /mayhem-consul
WORKDIR /mayhem-consul
RUN make dev

# Package Stage
FROM --platform=linux/amd64 ubuntu:20.04

COPY --from=builder /mayhem-consul/bin/consul /

