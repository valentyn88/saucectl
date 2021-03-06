package testcomposer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type Responder struct {
	Index   int
	Records []func(w http.ResponseWriter, r *http.Request)
	Test    *testing.T
}

func (r *Responder) Record(resFunc func(w http.ResponseWriter, req *http.Request)) {
	r.Records = append(r.Records, resFunc)
}

func (r *Responder) Play(w http.ResponseWriter, req *http.Request) {
	if r.Index >= len(r.Records) {
		r.Test.Errorf("responder requested more times than it has available records")
	}

	r.Records[r.Index](w, req)
	r.Index++
}

func respondJSON(w http.ResponseWriter, v interface{}) {
	w.WriteHeader(200)
	b, err := json.Marshal(v)

	if err != nil {
		log.Err(err).Msg("failed to marshal job json")
		http.Error(w, "failed to marshal job json", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write(b); err != nil {
		log.Err(err).Msg("Failed to write out response")
	}
}

func TestTestComposer_StartJob(t *testing.T) {
	respo := Responder{
		Test: t,
	}
	mockTestComposerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respo.Play(w, r)
	}))
	type args struct {
		ctx               context.Context
		jobStarterPayload JobStarterPayload
	}
	type fields struct {
		HTTPClient http.Client
		URL        string
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       string
		wantErr    error
		serverFunc func(w http.ResponseWriter, r *http.Request) // what shall the mock server respond with
	}{
		{
			name: "Happy path",
			fields: fields{
				HTTPClient: *mockTestComposerServer.Client(),
				URL:        mockTestComposerServer.URL,
			},
			args: args{
				ctx: context.TODO(),
				jobStarterPayload: JobStarterPayload{
					User:        "fake-user",
					AccessKey:   "fake-access-key",
					BrowserName: "fake-browser-name",
					TestName:    "fake-test-name",
					Framework:   "fake-framework",
					BuildName:   "fake-buildname",
					Tags:        nil,
				},
			},
			want:    "fake-job-id",
			wantErr: nil,
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				respondJSON(w, Job{
					ID:    "fake-job-id",
					Owner: "fake-owner",
				})
			},
		},
		{
			name: "Non 2xx status code",
			fields: fields{
				HTTPClient: *mockTestComposerServer.Client(),
				URL:        mockTestComposerServer.URL,
			},
			args: args{
				ctx:               context.TODO(),
				jobStarterPayload: JobStarterPayload{},
			},
			want:    "",
			wantErr: fmt.Errorf("Failed to start job. statusCode='300'"),
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(300)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				HTTPClient: tt.fields.HTTPClient,
				URL:        tt.fields.URL,
			}

			respo.Record(tt.serverFunc)

			got, err := c.StartJob(tt.args.ctx, tt.args.jobStarterPayload)
			if (err != nil) && !reflect.DeepEqual(err, tt.wantErr) {
				t.Errorf("StartJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StartJob() got = %v, want %v", got, tt.want)
			}
		})
	}
}
