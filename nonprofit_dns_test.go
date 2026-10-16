package main

import (
	"context"
	"reflect"
	"testing"
)

func TestDesiredRecords(t *testing.T) {
	input := ProgramDNS{
		Domain:              "giving.example.org",
		ReceiptMailServer:   "mail.example.net",
		VolunteerPortal:     "volunteers.example.net",
		CampaignReportToken: "campaign=autumn-2026",
	}
	wantPriority := 10
	tests := []struct {
		name string
		got  DNSRecord
		want DNSRecord
	}{
		{"donor receipts use prioritized MX", DesiredRecords(input, "zone_42")[0], DNSRecord{ZoneID: "zone_42", RecordType: "MX", Name: "receipts", Content: "mail.example.net", TTL: 3600, Priority: &wantPriority}},
		{"volunteer reminders use portal CNAME", DesiredRecords(input, "zone_42")[1], DNSRecord{ZoneID: "zone_42", RecordType: "CNAME", Name: "volunteer", Content: "volunteers.example.net", TTL: 300}},
		{"campaign reporting uses TXT", DesiredRecords(input, "zone_42")[2], DNSRecord{ZoneID: "zone_42", RecordType: "TXT", Name: "_campaign-report", Content: "campaign=autumn-2026", TTL: 300}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("record mismatch\ngot:  %#v\nwant: %#v", tt.got, tt.want)
			}
		})
	}
}

type recordingDNS struct {
	requestedDomain string
	records         []DNSRecord
}

func (r *recordingDNS) ZoneID(_ context.Context, domain string) (string, error) {
	r.requestedDomain = domain
	return "zone_nonprofit", nil
}

func (r *recordingDNS) UpsertRecord(_ context.Context, record DNSRecord) error {
	r.records = append(r.records, record)
	return nil
}

func TestSyncProgramDNSResolvesZoneBeforeWrites(t *testing.T) {
	api := &recordingDNS{}
	input := ProgramDNS{"giving.example.org", "mail.example.net", "volunteers.example.net", "campaign=spring"}
	result, err := SyncProgramDNS(context.Background(), api, input)
	if err != nil {
		t.Fatal(err)
	}
	if api.requestedDomain != input.Domain || result.ZoneID != "zone_nonprofit" || result.Count != 3 {
		t.Fatalf("unexpected sync result: domain=%q result=%+v", api.requestedDomain, result)
	}
	for _, record := range api.records {
		if record.ZoneID != result.ZoneID {
			t.Fatalf("record %q used zone %q", record.Name, record.ZoneID)
		}
	}
}
