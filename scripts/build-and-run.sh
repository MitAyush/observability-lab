# docker build -t payment-service:latest .

# docker run --rm  --name payment-service --log-driver=fluentd --log-opt fluentd-address=localhost:24224 --log-opt tag=docker.payment-service payment-service:latest

docker build -t payment-service:latest .

docker run --rm \
    --name payment-service \
    --log-driver=fluentd \
    --log-opt fluentd-address=localhost:24224 \
    --log-opt tag=docker.payment-service \
    payment-service:latest