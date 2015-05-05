// +build unit_test

package service

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/master/ports"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/mesos/mesos-go/auth/sasl"
	execcfg "github.com/mesosphere/kubernetes-mesos/pkg/executor/config"
	schedcfg "github.com/mesosphere/kubernetes-mesos/pkg/scheduler/config"
	"github.com/stretchr/testify/assert"
)

type fakeSchedulerProcess struct {
	doneFunc     func() <-chan struct{}
	failoverFunc func() <-chan struct{}
}

func (self *fakeSchedulerProcess) Terminal() <-chan struct{} {
	if self == nil || self.doneFunc == nil {
		return nil
	}
	return self.doneFunc()
}

func (self *fakeSchedulerProcess) Failover() <-chan struct{} {
	if self == nil || self.failoverFunc == nil {
		return nil
	}
	return self.failoverFunc()
}

func (self *fakeSchedulerProcess) End() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func Test_awaitFailoverDone(t *testing.T) {
	done := make(chan struct{})
	p := &fakeSchedulerProcess{
		doneFunc: func() <-chan struct{} { return done },
	}
	ss := &SchedulerServer{}
	failoverHandlerCalled := false
	failoverFailedHandler := func() error {
		failoverHandlerCalled = true
		return nil
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ss.awaitFailover(p, failoverFailedHandler)
	}()
	close(done)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for failover")
	}
	if failoverHandlerCalled {
		t.Fatalf("unexpected call to failover handler")
	}
}

func Test_awaitFailoverDoneFailover(t *testing.T) {
	ch := make(chan struct{})
	p := &fakeSchedulerProcess{
		doneFunc:     func() <-chan struct{} { return ch },
		failoverFunc: func() <-chan struct{} { return ch },
	}
	ss := &SchedulerServer{}
	failoverHandlerCalled := false
	failoverFailedHandler := func() error {
		failoverHandlerCalled = true
		return nil
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- ss.awaitFailover(p, failoverFailedHandler)
	}()
	close(ch)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for failover")
	}
	if !failoverHandlerCalled {
		t.Fatalf("expected call to failover handler")
	}
}

func Test_buildFrameworkInfo(t *testing.T) {
	assert := assert.New(t)

	schedServer := SchedulerServer{
		Port:                   ports.SchedulerPort,
		Address:                util.IP(net.ParseIP("127.0.0.1")),
		FailoverTimeout:        time.Duration((1 << 62) - 1).Seconds(),
		ExecutorRunProxy:       true,
		ExecutorSuicideTimeout: execcfg.DefaultSuicideTimeout,
		MesosAuthProvider:      sasl.ProviderName,
		MesosUser:              defaultMesosUser,
		ReconcileInterval:      defaultReconcileInterval,
		ReconcileCooldown:      defaultReconcileCooldown,
		Checkpoint:             true,
		FrameworkName:          schedcfg.DefaultInfoName,
		HA:                     false,
		mux:                    http.NewServeMux(),
		KubeletCadvisorPort:    4194, // copied from github.com/GoogleCloudPlatform/kubernetes/blob/release-0.14/cmd/kubelet/app/server.go
		KubeletSyncFrequency:   10 * time.Second,
	}

	info, cred, err := schedServer.buildFrameworkInfo()

	assert.Equal(nil, err)

	assert.Equal(*info.Name, schedcfg.DefaultInfoName)
	assert.Equal(*info.User, defaultMesosUser)
	username, err2 := schedServer.getUsername()
	assert.Equal(nil, err2)
	assert.Equal(*info.User, username)
	assert.Equal(*info.Checkpoint, true)
	assert.Equal(*info.FailoverTimeout, time.Duration((1<<62)-1).Seconds())
	assert.Nil(info.Role)

	_ = cred
}
