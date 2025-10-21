# --- Build ---

FROM golang:1.25.2-trixie AS build
WORKDIR /app
COPY . .

# Setup Compiler
RUN apt-get update
RUN apt-get install -y gcc wget tar
RUN rm -rf /var/lib/apt/lists/*

# Setup Tensorflow
RUN wget -O tensorflow.tar.gz -q https://storage.googleapis.com/tensorflow/versions/2.18.0/libtensorflow-cpu-linux-x86_64.tar.gz && \
    tar -C /usr/local -xzf tensorflow.tar.gz && \
    rm tensorflow.tar.gz

# Build Application
ENV CGO_ENABLED=1 \
    CGO_CFLAGS="-I/usr/local/include" \
    CGO_LDFLAGS="-L/usr/local/lib -ltensorflow"

RUN go build -o main.elf main.go

# --- Runtime ---

FROM debian:trixie-slim AS runtime
WORKDIR /app
RUN mkdir -p /data

# Install Dependencies
RUN apt-get update
RUN apt-get install -y ffmpeg
RUN rm -rf /var/lib/apt/lists/*

# Copy Application
COPY --from=build /usr/local/lib /usr/local/lib
COPY --from=build /app/resources ./resources
COPY --from=build /app/main.elf .

# Update Environment
ENV LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH
ENV TF_CPP_MIN_LOG_LEVEL=2
ENV HTTP_ADDRESS="0.0.0.0:8080"
ENV DATA_DIRECTORY=/data

# Start Application
EXPOSE 8080
CMD ["./main.elf"]

