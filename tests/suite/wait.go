package suite

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	containerStatusCreated    = "created"
	containerStatusRunning    = "running"
	containerStatusPaused     = "paused"
	containerStatusRestarting = "restarting"
	containerStatusRemoving   = "removing"
	containerStatusExited     = "exited"
	containerStatusDead       = "dead"

	codeCouldNotGetState = 10001
	codeCouldNotGetLogs  = 10002
	codeCouldNotReadLogs = 10003
	codeUnhealthy        = 10004
	codeUnhealthyNoLogs  = 10005
	codeNotRunning       = 10006
)

// Implement interface.
var _ wait.Strategy = (*CmdStrategy)(nil)

// CmdStrategy is a strategy to check if a container is healthy.
type CmdStrategy struct {
	execTimeout  time.Duration
	execInterval time.Duration
	retries      int64

	cmd  string
	args []string
}

// WithRetries sets a number of retries.
func (c *CmdStrategy) WithRetries(retries int) *CmdStrategy {
	c.retries = int64(retries)

	return c
}

// WithExecTimeout sets the execution timeout.
func (c *CmdStrategy) WithExecTimeout(timeout time.Duration) *CmdStrategy {
	c.execTimeout = timeout

	return c
}

// WithExecInterval sets the polling interval for each execution.
func (c *CmdStrategy) WithExecInterval(interval time.Duration) *CmdStrategy {
	c.execInterval = interval

	return c
}

// WaitUntilReady checks whether a container is healthy.
func (c *CmdStrategy) WaitUntilReady(ctx context.Context, target wait.StrategyTarget) error {
	timeout := time.Duration(int64(c.execTimeout+c.execInterval) * c.retries)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return timeoutCtx.Err()

		default:
			code, err := c.exec(ctx, target)
			if err != nil {
				return err
			}

			if code != 0 {
				time.Sleep(c.execInterval)

				continue
			}

			return nil
		}
	}
}

func (c *CmdStrategy) exec(ctx context.Context, target wait.StrategyTarget) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.execTimeout)
	defer cancel()

	state, err := target.State(ctx)
	if err != nil {
		return codeCouldNotGetState, err
	}

	if isContainerUnhealthy(state.Status) {
		logs, err := target.Logs(context.Background())
		if err != nil {
			return codeCouldNotGetLogs, fmt.Errorf("container is unhealthy and unable to get logs: %w", err)
		}

		if logs != nil {
			out, err := io.ReadAll(logs)

			if err != nil {
				return codeCouldNotReadLogs, fmt.Errorf("container is unhealthy and unable to read logs: %w", err)
			}

			return codeUnhealthy, fmt.Errorf("container is unhealthy, logs:\n%s", string(out))
		}

		return codeUnhealthyNoLogs, fmt.Errorf("container is unhealthy and no logs")
	}

	if !state.Running {
		return codeNotRunning, nil
	}

	cmd := make([]string, 0, len(c.args)+1)
	cmd = append(cmd, c.cmd)
	cmd = append(cmd, c.args...)

	return target.Exec(ctx, cmd)
}

// WaitForCmd constructs a strategy waiting for an execution result.
func WaitForCmd(cmd string, args ...string) *CmdStrategy {
	return &CmdStrategy{
		cmd:  cmd,
		args: args,

		retries:      3,
		execTimeout:  5 * time.Second,
		execInterval: 10 * time.Second,
	}
}

func isContainerUnhealthy(status string) bool {
	return status != containerStatusCreated &&
		status != containerStatusRunning &&
		status != containerStatusRestarting
}

var _ wait.Strategy = (*TimeStrategy)(nil)

// TimeStrategy is a strategy to check if a container is healthy.
type TimeStrategy struct {
	duration time.Duration
}

// WaitUntilReady checks whether a container is healthy.
func (t *TimeStrategy) WaitUntilReady(ctx context.Context, target wait.StrategyTarget) error {
	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-time.After(t.duration):
		return nil
	}
}

// WaitForDuration constructs a strategy waiting for a given time.
func WaitForDuration(d time.Duration) *TimeStrategy {
	return &TimeStrategy{duration: d}
}
