package dmsgpty

// LocalPtyGateway is the gateway to a local pty.
type LocalPtyGateway struct {
	ses *Pty
}

// NewPtyGateway creates a new gateway to a local pty.
func NewPtyGateway(ses *Pty) PtyGateway {
	return &LocalPtyGateway{ses: ses}
}

// Stop stops the local pty.
func (g *LocalPtyGateway) Stop(_, _ *struct{}) error {
	return g.ses.Stop()
}

// Read reads from the local pty.
func (g *LocalPtyGateway) Read(reqN *int, respB *[]byte) error {
	b := make([]byte, *reqN)
	n, err := g.ses.Read(b)
	*respB = b[:n]
	return err
}

// Start starts the local pty.
func (g *LocalPtyGateway) Start(req *CommandReq, _ *struct{}) error {
	return g.ses.Start(req.Name, req.Arg, req.Size)
}

// Write writes to the local pty.
func (g *LocalPtyGateway) Write(wb *[]byte, n *int) error {
	var err error
	*n, err = g.ses.Write(*wb)
	return err
}

// ProxiedPtyGateway is an RPC gateway for a remote pty.
type ProxiedPtyGateway struct {
	ptyC *PtyClient
}

// NewProxyGateway creates a new pty-proxy gateway
func NewProxyGateway(ptyC *PtyClient) PtyGateway {
	return &ProxiedPtyGateway{ptyC: ptyC}
}

// Start starts the remote pty.
func (g *ProxiedPtyGateway) Start(req *CommandReq, _ *struct{}) error {
	return g.ptyC.Start(req.Name, req.Arg...)
}

// Stop stops the remote pty.
func (g *ProxiedPtyGateway) Stop(_, _ *struct{}) error {
	return g.ptyC.Stop()
}

// Read reads from the remote pty.
func (g *ProxiedPtyGateway) Read(reqN *int, respB *[]byte) error {
	b := make([]byte, *reqN)
	n, err := g.ptyC.Read(b)
	*respB = b[:n]
	return err
}

// Write writes to the remote pty.
func (g *ProxiedPtyGateway) Write(reqB *[]byte, respN *int) error {
	var err error
	*respN, err = g.ptyC.Write(*reqB)
	return err
}
