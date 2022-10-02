# failoverd

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