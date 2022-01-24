// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package reporttypes contains the types that are also defined in the browser clients.
// The formats need to be consistent.
package reporttypes

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"google.golang.org/protobuf/proto"
	pb "github.com/google/privacy-sandbox-aggregation-service/encryption/crypto_go_proto"
)

const (
	// Protocols of the report.
	onepartyProtocol = "one-party"
	mpcProtocol      = "mpc"
)

// The struct tags in the following structs need to be consistent with the field names defined in:
// https://github.com/WICG/conversion-measurement-api/blob/main/AGGREGATE.md#aggregate-attribution-reports

// AggregationServicePayload contains the payload for a specific aggregation server.
type AggregationServicePayload struct {
	// Payload is a encrypted CBOR serialized instance of struct Payload, which is base64 encoded.
	Payload string `json:"payload"`
	KeyID   string `json:"key_id"`
	// Debug cleartext payload is empty for non-debug reports.
	DebugCleartextPayload string `json:"debug_cleartext_payload"`
}

// AggregatableReport contains the information generated by the browser from a key-value pair,
// which will be used for server-side aggregation.
type AggregatableReport struct {
	SourceSite             string `json:"source_site"`
	AttributionDestination string `json:"attribution_destination"`
	// SharedInfo is a JSON serialized instance of struct SharedInfo.
	// This exact string is used as authenticated data for decryption. The string
	// therefore must be forwarded to the aggregation service unmodified. The
	// reporting origin can parse the string to access the encoded fields.
	// https://github.com/WICG/conversion-measurement-api/blob/main/AGGREGATE.md#aggregatable-reports
	SharedInfo                 string                       `json:"shared_info"`
	AggregationServicePayloads []*AggregationServicePayload `json:"aggregation_service_payloads"`

	// Debug keys are empty for non-debug reports.
	SourceDebugKey  string `json:"source_debug_key"`
	TriggerDebugKey string `json:"trigger_debug_key"`
}

// SharedInfo contains the shared infomation that will be used as the context info for the hybrid encryption.
type SharedInfo struct {
	ScheduledReportTime    string `json:"scheduled_report_time"`
	PrivacyBudgetKey       string `json:"privacy_budget_key"`
	Version                string `json:"version"`
	ReportID               string `json:"report_id"`
	ReportingOrigin        string `json:"reporting_origin"`
	SourceRegistrationTime string `json:"source_registration_time"`
	DebugMode              bool   `json:"debug_mode"`
}

// Contribution contains a single histogram contribution.
type Contribution struct {
	Bucket []byte `json:"bucket"`
	Value  []byte `json:"value"`
}

// Payload defines the payload sent to one server. This type is CBOR-serialized and contained by struct AggregationServicePayload.
type Payload struct {
	Operation string `json:"operation"`
	// For the MPC protocol, each histogram contribution is encrypted into two DPFKeys, which is a serialized proto of:
	// https://github.com/google/distributed_point_functions/blob/199696c7cde95d9f9e07a4dddbcaaa36d120ca12/dpf/distributed_point_function.proto#L110
	DPFKey []byte `json:"dpf_key"`
	// For the one-party protocol, the contribution is stored in clear text.
	Data []Contribution `json:"data"`
}

// GetProtocol gets the protocol which the report uses.
func (r *AggregatableReport) GetProtocol() (string, error) {
	var protocol string
	switch len(r.AggregationServicePayloads) {
	case 1:
		protocol = onepartyProtocol
	case 2:
		protocol = mpcProtocol
	default:
		return "", fmt.Errorf("expect 1 or 2 payloads, got %d", len(r.AggregationServicePayloads))
	}
	return protocol, nil
}

// Validate checks if a report is valid.
func (r *AggregatableReport) Validate() error {
	if got := len(r.AggregationServicePayloads); got != 1 && got != 2 {
		return fmt.Errorf("expected one or two payloads, got %d", got)
	}

	return nil
}

// IsDebugReport checks if a report has a clear text debug payload.
func (r *AggregatableReport) IsDebugReport() bool {
	return r.AggregationServicePayloads[0].DebugCleartextPayload != ""
}

// ExtractPayloadsFromAggregatableReport extracts records to be processed by the aggregators.
func (r *AggregatableReport) ExtractPayloadsFromAggregatableReport(useCleartext bool) ([]*pb.AggregatablePayload, error) {
	var output []*pb.AggregatablePayload
	for _, payload := range r.AggregationServicePayloads {
		var (
			data  []byte
			err   error
			keyID string
		)
		if useCleartext {
			data, err = base64.StdEncoding.DecodeString(payload.DebugCleartextPayload)
		} else {
			data, err = base64.StdEncoding.DecodeString(payload.Payload)
			keyID = payload.KeyID
		}
		if err != nil {
			return nil, err
		}
		output = append(output, &pb.AggregatablePayload{
			Payload:    &pb.StandardCiphertext{Data: data},
			SharedInfo: r.SharedInfo,
			KeyId:      keyID,
		})
	}
	return output, nil
}

func (r *AggregatableReport) convertReport(useCleartext bool) (map[string]string, error) {
	payloads, err := r.ExtractPayloadsFromAggregatableReport(useCleartext)
	if err != nil {
		return nil, err
	}

	output := make(map[string]string)
	for index, record := range payloads {
		payload, err := SerializeAggregatablePayload(record)
		if err != nil {
			return nil, err
		}
		output[strconv.Itoa(index)] = payload
	}
	return output, nil
}

// GetSerializedEncryptedRecords extracts and serializes the encrypted payloads.
func (r *AggregatableReport) GetSerializedEncryptedRecords() (map[string]string, error) {
	return r.convertReport(false /*useCleartext*/)
}

// GetSerializedCleartextRecords extracts and serializes the cleartext payloads.
func (r *AggregatableReport) GetSerializedCleartextRecords() (map[string]string, error) {
	return r.convertReport(true /*useCleartext*/)
}

// SerializeAggregatablePayload serializes the AggregatablePayload into a string.
func SerializeAggregatablePayload(encrypted *pb.AggregatablePayload) (string, error) {
	bEncrypted, err := proto.Marshal(encrypted)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bEncrypted), nil
}

// DeserializeAggregatablePayload deserializes the AggregatablePayload from a string.
func DeserializeAggregatablePayload(line string) (*pb.AggregatablePayload, error) {
	bsc, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return nil, err
	}

	payload := &pb.AggregatablePayload{}
	if err := proto.Unmarshal(bsc, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
