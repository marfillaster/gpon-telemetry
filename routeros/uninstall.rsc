# Remove GPON telemetry RouterOS objects. Data files on your storage volume
# are left in place intentionally.

/system scheduler remove [find name="gpon-container-poll"]
/system script remove [find name="gpon-container-poll"]
/system logging remove [find action="gponusb"]
/system logging action remove [find name="gponusb"]
/container stop [find hostname="gpontelemetry"]
:delay 2s
/container remove [find hostname="gpontelemetry"]
/container/mounts remove [find list="gpon-telemetry-mounts"]
/container/envs remove [find list="gpon-telemetry-envs"]
/interface/bridge/port remove [find interface="veth-gpontelemetry"]
/interface/veth remove [find name="veth-gpontelemetry"]
