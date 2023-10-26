// source taken from https://git.prolicht.digital/golib/healthcheck/-/blob/master/healthcheck_test.go

package healthcheck

import (
	"context"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	type args struct {
		opts []Option
	}
	tests := []struct {
		name string
		args args
		want *service
	}{
		{
			name: "Test Plain",
			args: args{},
			want: &service{
				listenAddress: ":11223",
				checkFunc:     nil, // we cannot compare the check function
			},
		},
		{
			name: "Test With Empty Options",
			args: args{
				opts: []Option{},
			},
			want: &service{
				listenAddress: ":11223",
				checkFunc:     nil, // we cannot compare the check function
			},
		},
		{
			name: "Test With Options",
			args: args{
				opts: []Option{ListenOn(":123456")},
			},
			want: &service{
				listenAddress: ":123456",
				checkFunc:     nil, // we cannot compare the check function
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.args.opts...)
			got.checkFunc = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListenOn(t *testing.T) {
	type args struct {
		addr string
	}
	tests := []struct {
		name string
		args args
		want *service
	}{
		{
			name: "Test Port Only",
			args: args{
				addr: ":8080",
			},
			want: &service{
				listenAddress: ":8080",
				checkFunc:     nil, // cannot deeply compare check func,
			},
		},
		{
			name: "Test Addr:Port Only",
			args: args{
				addr: "localhost:8080",
			},
			want: &service{
				listenAddress: "localhost:8080",
				checkFunc:     nil, // cannot deeply compare check func,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(ListenOn(tt.args.addr))
			got.checkFunc = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListenOn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListenOnEnv(t *testing.T) {
	_ = os.Setenv("HC_LISTEN_ADDR", "")
	hc := New(ListenOnFromEnv())
	if hc.listenAddress != New().listenAddress {
		t.Errorf("ListenOnFromEnv() = %v, want %v", hc.listenAddress, New().listenAddress)
	}

	want := ":1337"
	_ = os.Setenv("HC_LISTEN_ADDR", want)
	hc = New(ListenOnFromEnv())
	if hc.listenAddress != want {
		t.Errorf("ListenOnFromEnv() = %v, want %v", hc.listenAddress, want)
	}

	hc = New() // check that the env var has no effect
	if hc.listenAddress != New().listenAddress {
		t.Errorf("ListenOnFromEnv() = %v, want %v", hc.listenAddress, New().listenAddress)
	}

	want = ":1338"
	_ = os.Setenv("SOME_RANDOM_ENV_VAR", want)
	hc = New(ListenOnFromEnv("SOME_RANDOM_ENV_VAR"))
	if hc.listenAddress != want {
		t.Errorf("ListenOnFromEnv() = %v, want %v", hc.listenAddress, want)
	}

	hc = New(ListenOnFromEnv("SOME_RANDOM_ENV_VAR", "ignored", "ignored 2"))
	if hc.listenAddress != want {
		t.Errorf("ListenOnFromEnv() = %v, want %v", hc.listenAddress, want)
	}
}

func TestWithCustomCheck(t *testing.T) {
	customFnc := func() int { return 123 }

	type args struct {
		fnc func() int
	}
	tests := []struct {
		name    string
		args    args
		want    *service
		wantFnc func() int
	}{
		{
			name: "Test Custom Function",
			args: args{
				fnc: customFnc,
			},
			want: &service{
				listenAddress: New().listenAddress,
				checkFunc:     nil, // cannot deeply compare check func,
			},
			wantFnc: customFnc,
		},
		{
			name: "Test Nil Function",
			args: args{
				fnc: nil,
			},
			want: &service{
				listenAddress: New().listenAddress,
				checkFunc:     nil, // cannot deeply compare check func,
			},
			wantFnc: New().checkFunc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(WithCustomCheck(tt.args.fnc))
			gotFnc := got.checkFunc
			got.checkFunc = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WithContext() = %v, want %v", got, tt.want)
			}

			if reflect.ValueOf(gotFnc).Pointer() != reflect.ValueOf(tt.wantFnc).Pointer() {
				t.Error("WithContext() function mismatch")
			}
		})
	}
}

func Test_service_StartForeground(t *testing.T) {
	runTime := 550 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	hc := New()
	start := time.Now()
	hc.StartForeground(ctx)
	elapsed := time.Since(start)

	// check if execution time is within +-10% of the runTime
	if elapsed > (runTime+(runTime/10)) || elapsed < (runTime-(runTime/10)) {
		t.Errorf("StartForeground() invalid execution time = %v, want %v", elapsed, runTime)
	}
}

func Test_service_HTTPResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	hc := New()
	hc.StartWithContext(ctx)
	time.Sleep(200 * time.Millisecond) // ensure that web server is up and running

	cl := http.Client{Timeout: time.Millisecond * 200}
	req, _ := http.NewRequest("GET", "http://localhost:11223/health", nil)
	resp, err := cl.Do(req)
	if err != nil {
		t.Errorf("http request failed:  %v", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("http request with wrong response code:  %v, want %v", err, http.StatusOK)
	}

	<-ctx.Done() // wait for clean shutdown
}

func Test_service_CustomCheckResponse(t *testing.T) {
	want := http.StatusExpectationFailed
	hc := New(WithCustomCheck(func() int {
		return want
	}))
	hc.Start()
	time.Sleep(200 * time.Millisecond) // ensure that web server is up and running

	cl := http.Client{Timeout: time.Millisecond * 200}
	req, _ := http.NewRequest("GET", "http://localhost:11223/health", nil)
	resp, err := cl.Do(req)
	if err != nil {
		t.Errorf("http request failed:  %v", err)
		return
	}
	if resp.StatusCode != want {
		t.Errorf("http request with wrong response code:  %v, want %v", err, want)
	}
}
