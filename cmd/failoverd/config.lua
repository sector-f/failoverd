ping_frequency = 1   -- Send a ping to each probe every 1 second
update_frequency = 5 -- Call on_update every 5 seconds
privileged = true    -- Use ICMP pings (rather than UDP)
num_seconds = 10     -- Pass on_update the stats for the probe with the lowest packet loss in the last 10 seconds

-- List of endpoints to ping
probes = {
    -- "192.168.0.1",
    "192.168.0.2",
    "192.168.0.3",
}

-- Gets called every update_frequency seconds
function on_update(ps)
    io.write(string.format("%s: %.2f\n", ps:dst(), ps:loss()))
end

function on_quit()
    print("Shutting down")
end