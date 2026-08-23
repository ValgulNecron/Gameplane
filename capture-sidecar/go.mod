module github.com/ValgulNecron/gameplane/capture-sidecar

go 1.26.0

require (
	github.com/ValgulNecron/gameplane/svcutil v0.0.0
	github.com/gopacket/gopacket v1.6.1
	github.com/packetcap/go-pcap v0.0.0-20260731105150-c86974bbfbcd
	golang.org/x/net v0.57.0
)

// svcutil is an in-repo module (no published version); resolve it locally
// both inside the workspace (go.work) and in standalone module/Docker builds.
replace github.com/ValgulNecron/gameplane/svcutil => ../svcutil

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
