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
}

-- Gets called whenever a response is received
function on_recv(gps, ps)
   io.write(string.format("%s: %.2f\n", ps:dst(), ps:loss()))
end

-- Gets called every update_frequency seconds
function on_update(gps)
    ps = gps:lowest_loss()
    io.write(string.format("%s: %.2f\n", ps:dst(), ps:loss()))
end

-- Gets called on program shutdown (SIGINT)
function on_quit(gps)
    print("Shutting down")
end
```

### Variables

* `ping_frequency`: seconds to wait before sending each packet. Default is `1`. (number)
* `update_frequency`: seconds to wait before calling on_update function. Default is `1`. (number)
* `privileged`: use ICMP pings if true, UDP pings if false. Default is `false`. (boolean)
* `num_seconds`: `failoverd` will keep track of packet loss for this number of seconds. Default is `10`. (number) 
* `probes`: list of probes to ping (array of `probe` objects)

Note that if `privileged` is `true`, then you will need to give `failoverd` the `CAP_NET_RAW` capability to allow it to send ICMP ping requests, unless you are running it as the superuser.

### Types

The following types are implemented for use in the configuration file:

#### probe

The `probe` type is used to specify endpoints that should be pinged. It is created via the `probe.new` function, which can be called in two ways:

* `probe.new(string)` creates a probe where the argument to `new()` is the destination IP address to ping
*  `probe.new(string, string)` creates a probe where the first argument to `new()` is the destination IP address to ping and the second argument is either the source IP address or the network interface whose IP address hould be used as the source

Additionally, probes can be started/stopped during runtime:

* `probe.start(probe)` starts a new probe
* `probe.stop(string)` stops the probe with the destination IP address specified by its argument

#### global_probe_stats

The `global_probe_stats` type stores information about the statistics of all running probes.
It has the following methods:

* `global_probe_stats::lowest_loss()` returns the `probe_stats` of the probe with the lowest packet loss
* `global_probe_stats::get(string)` uses its argument as a destination address and returns the corresponding `probe_stats`

#### probe_stats

The `probe_stats` type stores information about the statistics about one running probe.
It has the following methods:

* `probe_stats::src()` returns the probe's source address
* `probe_stats::dst()` returns the probe's destination address
* `probe_stats::loss()` returns the probe's current packet loss as a number from 0-100 (percent)

### Functions

The following functions can be specified in the configuration file; they will be called by `failoverd` when indicated. Note that all functions are optional.

* `on_recv(global_probe_stats, probe_stats)` is called whenever a ping response is received from any endpoint. `probe_stats` is the statistics corresponding to the probe for which a response was received.
* `on_update(global_probe_stats)` is called every `update_frequency` seconds
* `on_quit(global_probe_stats)` is called when the program exits (due to SIGINT)