# Telemetry

cliamp sends a single anonymous ping once per calendar month so we can count monthly active users.

## What is collected

- A randomly generated UUID (created on first launch, stored locally)
- The app version string

That's it. No IP logging, no usage data, no personal information.

## How it works

1. On first launch, a random UUID is generated and saved to `~/.config/cliamp/.telemetry_id`
2. Each launch checks if a ping has already been sent this month
3. If not, a single background POST request is sent to `https://telemetry.cliamp.stream/ping`
4. The request is fire-and-forget — it never blocks the app or surfaces errors

## Opting out

You can disable telemetry entirely using either method:

### Config file (permanent)

Add to `~/.config/cliamp/config.toml`:

```toml
telemetry = false
```

### CLI flag (per session)

```sh
cliamp --no-telemetry
```

When telemetry is disabled, no network requests are made and no telemetry state file is created or updated.

## Storage

The telemetry state file is located at:

```
~/.config/cliamp/.telemetry_id
```

It contains the UUID and the last ping month in JSON format.
