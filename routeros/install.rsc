# GPON telemetry install for RouterOS 7.x containers.
#
# Before import:
#   1. Build and upload gpon-telemetry.tar to your RouterOS storage volume.
#   2. Adjust addresses/passwords below for your network.
#   3. Import with: /import file-name=install.rsc

:local containerName "gpontelemetry"
:local storageRoot "<storage-volume>"
:local imageFile ($storageRoot . "/gpon-telemetry.tar")
:local logDir ($storageRoot . "/gpontelemetry/logs")
:local vethName "veth-gpontelemetry"
:local bridgeName "bridge"
:local containerIPv4 "<container-ipv4-cidr>"
:local gatewayIPv4 "<router-ipv4>"
:local containerIPv6 ""
:local gatewayIPv6 ""
:local stickHost "<gpon-stick-host>"
:local stickUser "admin"
:local stickPass "admin"
:local actionName "gponusb"
:local rawLogBase ($logDir . "/gpon")
:local pollInterval "5m"

/file
:if ([:len [find name=($storageRoot . "/gpontelemetry")]] = 0) do={
  add name=($storageRoot . "/gpontelemetry") type=directory
}
:if ([:len [find name=$logDir]] = 0) do={
  add name=$logDir type=directory
}

/interface/veth
:if ([:len [find name=$vethName]] = 0) do={
  add name=$vethName address=$containerIPv4 gateway=$gatewayIPv4
}
:if ([:len $containerIPv6] > 0) do={
  set [find name=$vethName] address=($containerIPv4 . "," . $containerIPv6) \
    gateway=$gatewayIPv4 gateway6=$gatewayIPv6
}

/interface/bridge/port
:if ([:len [find interface=$vethName bridge=$bridgeName]] = 0) do={
  add bridge=$bridgeName interface=$vethName
}

/container/mounts
:if ([:len [find list="gpon-telemetry-mounts" dst="/var/lib/gpontelemetry"]] = 0) do={
  add list="gpon-telemetry-mounts" src=("/" . $logDir) dst="/var/lib/gpontelemetry" \
    read-only=no
} else={
  set [find list="gpon-telemetry-mounts" dst="/var/lib/gpontelemetry"] src=("/" . $logDir) \
    read-only=no
}

/container/envs
:foreach k in={"GPON_HOST";"GPON_USER";"GPON_PASS";"GPON_LOG_ROOT";"GPON_STATIC_ROOT";"GPON_ROOT";"GPON_ADDR";"GPON_STICK_URL"} do={
  :do { remove [find list="gpon-telemetry-envs" key=$k] } on-error={}
}
add list="gpon-telemetry-envs" key=GPON_HOST value=$stickHost
add list="gpon-telemetry-envs" key=GPON_USER value=$stickUser
add list="gpon-telemetry-envs" key=GPON_PASS value=$stickPass
add list="gpon-telemetry-envs" key=GPON_LOG_ROOT value="/var/lib/gpontelemetry"
add list="gpon-telemetry-envs" key=GPON_STATIC_ROOT value="/opt/gpontelemetry/www"
add list="gpon-telemetry-envs" key=GPON_ADDR value=":3000"

/system logging action
:if ([:len [find name=$actionName]] = 0) do={
  add name=$actionName target=disk disk-file-name=$rawLogBase \
    disk-lines-per-file=288 disk-file-count=1 disk-stop-on-full=no
} else={
  set [find name=$actionName] target=disk disk-file-name=$rawLogBase \
    disk-lines-per-file=288 disk-file-count=1 disk-stop-on-full=no
}

/system logging
:if ([:len [find action=$actionName]] = 0) do={
  add topics=script regex="GPONRAW" action=$actionName
} else={
  set [find action=$actionName] topics=script regex="GPONRAW" action=$actionName
}

/container
:do { stop [find hostname=$containerName] } on-error={}
:delay 2s
:do { remove [find hostname=$containerName] } on-error={}
add file=$imageFile interface=$vethName mountlists="gpon-telemetry-mounts" \
  envlists="gpon-telemetry-envs" hostname=$containerName logging=yes \
  start-on-boot=yes
start [find hostname=$containerName]

/system script
:if ([:len [find name="gpon-container-poll"]] = 0) do={
  add name="gpon-container-poll" policy=read,write,test source={
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
} else={
  set [find name="gpon-container-poll"] source={
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
}

/system scheduler
:if ([:len [find name="gpon-container-poll"]] = 0) do={
  add name="gpon-container-poll" interval=$pollInterval start-time=startup \
    on-event="gpon-container-poll" policy=read,write,test \
    comment="HTTP-poll GPON stick and roll up inside container"
} else={
  set [find name="gpon-container-poll"] interval=$pollInterval start-time=startup \
    on-event="gpon-container-poll" policy=read,write,test \
    comment="HTTP-poll GPON stick and roll up inside container"
}
