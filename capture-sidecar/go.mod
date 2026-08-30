module github.com/ValgulNecron/gameplane/capture-sidecar

go 1.26.0

require (
	github.com/gopacket/gopacket v1.7.1
	github.com/packetcap/go-pcap v0.0.0-20260731105150-c86974bbfbcd
	golang.org/x/net v0.58.0
)

// svcutil is an in-repo module (no published version); resolve it locally
// both inside the workspace (go.work) and in standalone module/Docker builds.
replace github.com/ValgulNecron/gameplane/svcutil => ../svcutil

require golang.org/x/sys v0.47.0 // indirect
