# CLI output

The terminal hierarchy follows the [official Speedtest CLI example](https://speedtest-static-dev.speedtest.dev/s/images/speedtest/apps/cli/cli-hero-1x.png). The official downloadable CLI manual also documents server selection (`-s`), output format (`-f`) and progress (`--progress yes|no`). GlobalSpeed implements those common entry points with its own protocol and result schema.

```text
   GlobalSpeed

      Server: 南京电信 - 江苏 · 南京 (id: 1503)
         ISP: 电信
 External IP: 203.0.113.9
Idle Latency:    29.28 ms   (jitter: 1.40 ms, ICMP)
    Download:    92.00 Mbps (data used: 0.50 MiB)
      Upload:    81.00 Mbps (data used: 0.50 MiB)
 Packet Loss:      0.0%
```

This is illustrative output using a documentation IP address. Speeds are Mbps, payload sizes are MiB. Progress follows elapsed sampling time or the phase's payload budget, whichever is higher. Each completed phase uses the measured mean; its instantaneous progress value does not replace the final result.

Only interactive terminals display live updates. Pipes and redirected files receive plain text. JSON output has no banner or progress and preserves GlobalSpeed's existing field names and units. No public result URL is generated. TCP fallback reports TCP connection failures rather than treating them as ICMP packet loss; missing measurements appear as `Unavailable`.

Checks cover final values versus instantaneous samples, phase transitions, line cleanup, plain output, unavailable latency/loss, remote metadata control characters, argument validation, and six platform/architecture builds. A live node test used 1 MiB total payload and confirmed session release.
