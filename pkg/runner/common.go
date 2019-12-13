package runner

// Use consistent IP address ranges for both the data and the control subnet.
// _which_
var (
	controlSubnet  = "192.168.0.0/16"
	controlGateway = "192.168.0.1"
	// this gives us two public A blocks and a private A block: 8., 9., & 10.
	dataSubnet  = "8.0.0.0/5"
	dataGateway = "8.0.0.1"
)
