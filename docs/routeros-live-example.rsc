# Example values. Replace storage volume, addresses, and credentials.

/interface/veth
add name=veth-gpontelemetry address=<container-ipv4-cidr> gateway=<router-ipv4>

/container/mounts
add list=gpon-telemetry-mounts src=/<storage-volume>/gpontelemetry/logs \
  dst=/var/lib/gpontelemetry \
  read-only=no

/system logging action
add name=gponusb target=disk disk-file-name=<storage-volume>/gpontelemetry/logs/gpon \
  disk-lines-per-file=288 disk-file-count=1 disk-stop-on-full=no

/system logging
add topics=script regex=GPONRAW action=gponusb

/container
add file=<storage-volume>/gpon-telemetry.tar interface=veth-gpontelemetry \
  mountlists=gpon-telemetry-mounts hostname=gpontelemetry logging=yes \
  start-on-boot=yes

/system script
add name=gpon-container-poll policy=read,write,test source={
  :local c [/container find where hostname="gpontelemetry"]
  :if ([:len $c] = 0) do={
    :log warning "GPON HTTP poll skipped: container missing"
  } else={
    :local line [/container/shell $c cmd="/gpontelemetry sample" as-value]
    :if ([:typeof [:find $line "GPONRAW UNHEALTHY"]] = "nil") do={
      :log info $line
    } else={
      :log error $line
    }
    :delay 2s
    /container/shell $c cmd="/gpontelemetry all"
  }
}

/system scheduler
add name=gpon-container-poll interval=5m start-time=startup \
  on-event=gpon-container-poll policy=read,write,test \
  comment="HTTP-poll GPON stick and roll up inside container"
