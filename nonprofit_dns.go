package main

import (
	"context"
	"fmt"
)

type ProgramDNS struct {
	Domain              string
	ReceiptMailServer   string
	VolunteerPortal     string
	CampaignReportToken string
}

type DNSRecord struct {
	ZoneID     string `json:"zone_id"`
	RecordType string `json:"record_type"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	TTL        int    `json:"ttl"`
	Priority   *int   `json:"priority,omitempty"`
}

type SyncResult struct {
	ZoneID string
	Count  int
}

type DNSAPI interface {
	ZoneID(context.Context, string) (string, error)
	UpsertRecord(context.Context, DNSRecord) error
}

func DesiredRecords(input ProgramDNS, zoneID string) []DNSRecord {
	mailPriority := 10
	return []DNSRecord{
		{ZoneID: zoneID, RecordType: "MX", Name: "receipts", Content: input.ReceiptMailServer, TTL: 3600, Priority: &mailPriority},
		{ZoneID: zoneID, RecordType: "CNAME", Name: "volunteer", Content: input.VolunteerPortal, TTL: 300},
		{ZoneID: zoneID, RecordType: "TXT", Name: "_campaign-report", Content: input.CampaignReportToken, TTL: 300},
	}
}

func SyncProgramDNS(ctx context.Context, api DNSAPI, input ProgramDNS) (SyncResult, error) {
	zoneID, err := api.ZoneID(ctx, input.Domain)
	if err != nil {
		return SyncResult{}, fmt.Errorf("resolve zone: %w", err)
	}
	records := DesiredRecords(input, zoneID)
	for _, record := range records {
		if err := api.UpsertRecord(ctx, record); err != nil {
			return SyncResult{}, fmt.Errorf("upsert %s record %q: %w", record.RecordType, record.Name, err)
		}
	}
	return SyncResult{ZoneID: zoneID, Count: len(records)}, nil
}
