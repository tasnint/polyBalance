#!/bin/bash

echo "Starting backend servers..."
go run ./cmd/backend -port 8081 -name backend-1 &
go run ./cmd/backend -port 8082 -name backend-2 &

sleep 2

echo "Starting PolyBalance load balancer..."
go run ./cmd
