package listeners

import (
	"io"

	"golang.org/x/crypto/ssh/agent"
)

// ServeAgent does not own or close its transport, even when the client disconnects.
func serveAgentConnection(sshagent agent.Agent, conn io.ReadWriteCloser) error {
	defer conn.Close()
	return agent.ServeAgent(sshagent, conn)
}
