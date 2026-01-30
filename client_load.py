#!/usr/bin/env python3
"""
client_load.py - Load testing client for policy-service

Sends concurrent Plan RPCs to measure latency performance.

Requirements:
    pip install grpcio grpcio-tools numpy

Usage:
    python client_load.py --host localhost --port 50051 --requests 2000 --concurrent 50
"""

import argparse
import statistics
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import List

import grpc
import numpy as np

# Import generated protobuf code
# Run: python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. proto/planner.proto
try:
    from proto import planner_pb2
    from proto import planner_pb2_grpc
except ImportError:
    print("Error: Generated protobuf files not found.")
    print("Run: python -m grpc_tools.protoc -I. --python_out=. --grpc_python_out=. proto/planner.proto")
    exit(1)


@dataclass
class LatencyResult:
    """Result of a single RPC call."""
    success: bool
    latency_ms: float
    error: str = ""


def create_random_observation(channels: int = 3, height: int = 64, width: int = 64) -> planner_pb2.Observation:
    """Create a random observation for testing."""
    data = np.random.randn(channels * height * width).astype(np.float32).tolist()
    return planner_pb2.Observation(
        data=data,
        channels=channels,
        height=height,
        width=width,
    )


def send_plan_request(stub: planner_pb2_grpc.PathPlannerStub, robot_id: int) -> LatencyResult:
    """Send a single Plan RPC and measure latency."""
    obs = create_random_observation()
    request = planner_pb2.PlanRequest(robot_id=robot_id, obs=obs)

    start = time.perf_counter()
    try:
        response = stub.Plan(request, timeout=10.0)
        latency_ms = (time.perf_counter() - start) * 1000
        return LatencyResult(success=True, latency_ms=latency_ms)
    except grpc.RpcError as e:
        latency_ms = (time.perf_counter() - start) * 1000
        return LatencyResult(success=False, latency_ms=latency_ms, error=str(e))


def run_load_test(
    host: str,
    port: int,
    num_requests: int,
    max_concurrent: int,
) -> List[LatencyResult]:
    """Run load test with concurrent requests."""
    channel = grpc.insecure_channel(f"{host}:{port}")
    stub = planner_pb2_grpc.PathPlannerStub(channel)

    results: List[LatencyResult] = []

    print(f"Starting load test: {num_requests} requests with {max_concurrent} concurrent workers")
    print(f"Target: {host}:{port}")
    print("-" * 60)

    start_time = time.perf_counter()

    with ThreadPoolExecutor(max_workers=max_concurrent) as executor:
        futures = {
            executor.submit(send_plan_request, stub, i): i
            for i in range(num_requests)
        }

        completed = 0
        for future in as_completed(futures):
            result = future.result()
            results.append(result)
            completed += 1

            if completed % 100 == 0:
                print(f"Progress: {completed}/{num_requests} ({100*completed/num_requests:.1f}%)")

    total_time = time.perf_counter() - start_time
    channel.close()

    return results, total_time


def print_statistics(results: List[LatencyResult], total_time: float):
    """Print latency statistics."""
    successful = [r for r in results if r.success]
    failed = [r for r in results if not r.success]

    print("\n" + "=" * 60)
    print("LOAD TEST RESULTS")
    print("=" * 60)

    print(f"\nTotal requests:     {len(results)}")
    print(f"Successful:         {len(successful)} ({100*len(successful)/len(results):.1f}%)")
    print(f"Failed:             {len(failed)} ({100*len(failed)/len(results):.1f}%)")
    print(f"Total time:         {total_time:.2f}s")
    print(f"Throughput:         {len(results)/total_time:.1f} req/s")

    if successful:
        latencies = [r.latency_ms for r in successful]
        latencies.sort()

        print(f"\nLatency Statistics (ms):")
        print(f"  Min:              {min(latencies):.2f}")
        print(f"  Max:              {max(latencies):.2f}")
        print(f"  Mean:             {statistics.mean(latencies):.2f}")
        print(f"  Median:           {statistics.median(latencies):.2f}")
        print(f"  Std Dev:          {statistics.stdev(latencies):.2f}" if len(latencies) > 1 else "  Std Dev:          N/A")

        # Percentiles
        p50_idx = int(len(latencies) * 0.50)
        p90_idx = int(len(latencies) * 0.90)
        p95_idx = int(len(latencies) * 0.95)
        p99_idx = int(len(latencies) * 0.99)

        print(f"\nPercentiles (ms):")
        print(f"  p50:              {latencies[p50_idx]:.2f}")
        print(f"  p90:              {latencies[p90_idx]:.2f}")
        print(f"  p95:              {latencies[p95_idx]:.2f}")
        print(f"  p99:              {latencies[min(p99_idx, len(latencies)-1)]:.2f}")

    if failed:
        print(f"\nSample errors:")
        for r in failed[:5]:
            print(f"  - {r.error[:100]}")


def main():
    parser = argparse.ArgumentParser(description="Load test client for policy-service")
    parser.add_argument("--host", default="localhost", help="gRPC server host")
    parser.add_argument("--port", type=int, default=50051, help="gRPC server port")
    parser.add_argument("--requests", type=int, default=2000, help="Total number of requests")
    parser.add_argument("--concurrent", type=int, default=50, help="Max concurrent requests")
    args = parser.parse_args()

    results, total_time = run_load_test(
        host=args.host,
        port=args.port,
        num_requests=args.requests,
        max_concurrent=args.concurrent,
    )

    print_statistics(results, total_time)


if __name__ == "__main__":
    main()
