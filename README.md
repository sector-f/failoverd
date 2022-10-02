# failoverd

A scriptable daemon for updating routes based on packet loss to endpoints.

## Configuration

`failoverd` is configured using a Lua script that specifies a number of variables and functions.

### Example

```lua
-- Number of seconds to wait before sending each packet
ping_frequency = 1

-- Number of seconds to wait before calling on_update function
update_frequency = 5

-- Use ICMP pings if true, UDP pings if false
privileged = true 

-- The global_probe_stats that are passed to functions are the statistics for the last num_seconds seconds
num_seconds = 10

-- List of endpoints to ping
probes = {
    probe.new("192.168.0.1"),         -- Pings 192.168.0.1
    probe.new("192.168.0.2", "eth0"), -- Pings 192.168.0.2 using eth0's address
    probe.new("google.com"),          -- Resolves google.com and pings its address
}

-- Gets called whenever a response is received
function on_recv(gps)
   ps = gps:lowest_loss()
   io.write(string.format("%s: %.2f\n", ps:dst(), ps:loss()))
end

-- Gets called every update_frequency seconds
function on_update(gps)
    ps = gps:lowest_loss()
    io.write(string.format("%s: %.2f\n", ps:dst(), ps:loss()))
end

-- Gets called on program shutdown (SIGINT)
function on_quit()
    print("Shutting down")
end
```

### Types

The following types are implemented for use in the configuration file.

#### probe

The `probe` type is used to specify endpoints that should be pinged. It is created via the `probe.new` function, which can be called in two ways:

* `probe.new(string)` creates a probe where the argument to `new()` is the destination IP address to ping
*  `probe.new(string, string)` creates a probe where the first argument to `new()` is the destination IP address to ping and the second argument is either the source IP address or the network interface whose IP address hould be used as the source

#### global_probe_stats

The `global_probe_stats` type stores information about the statistics of all running probes.
It has the following methods:

* `global_probe_stats::lowest_loss()` returns the `probe_stats` of the probe with the lowest packet loss
* `global_probe_stats::get(string)` uses its argument as a destination address/hostname and returns the corresponding `probe_stats`

#### probe_stats

The `probe_stats` type stores information about the statistics about one running probe.
It has the following methods:

* `probe_stats::src()` returns the probe's source address
* `probe_stats::dst()` returns the probe's destination address
* `probe_stats::loss()` returns the probe's current packet loss as a number from 0-100 (percent)