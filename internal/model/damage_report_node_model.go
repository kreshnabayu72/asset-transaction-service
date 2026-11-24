package model

import "time"

// DamageReportNode is one Damage Log node; a nil ActorID means System.
type DamageReportNode struct {
	ID                   int64      `json:"id,string" column:"damage_report_nodes.id"`
	RecordID             int64      `json:"record_id,string" column:"damage_report_nodes.record_id"`
	NodeType             string     `json:"node_type" column:"damage_report_nodes.node_type"`
	ActorID              *int64     `json:"actor_id,string" column:"damage_report_nodes.actor_id"`
	ActorName            *string    `json:"actor_name" column:"damage_report_nodes.actor_name"`
	OnBehalfOfName       *string    `json:"on_behalf_of_name" column:"damage_report_nodes.on_behalf_of_name"`
	WhatHappened         *string    `json:"what_happened" column:"damage_report_nodes.what_happened"`
	Reason               *string    `json:"reason" column:"damage_report_nodes.reason"`
	RepairNote           *string    `json:"repair_note" column:"damage_report_nodes.repair_note"`
	Note                 *string    `json:"note" column:"damage_report_nodes.note"`
	DamagedPhotoFileID   *int64     `json:"damaged_photo_file_id,string" column:"damage_report_nodes.damaged_photo_file_id"`
	AfterPhotoFileID     *int64     `json:"after_photo_file_id,string" column:"damage_report_nodes.after_photo_file_id"`
	PhotoDeletedAt       *time.Time `json:"photo_deleted_at" column:"damage_report_nodes.photo_deleted_at"`
	ManualIdentification bool       `json:"manual_identification" column:"damage_report_nodes.manual_identification"`
	TagReportID          *int64     `json:"tag_report_id,string" column:"damage_report_nodes.tag_report_id"`
	ReportID             *string    `json:"report_id" column:"damage_report_nodes.report_id"`
	DisposalNo           *string    `json:"disposal_no" column:"damage_report_nodes.disposal_no"`
	OriginRefType        *string    `json:"origin_ref_type" column:"damage_report_nodes.origin_ref_type"`
	OriginRefNo          *string    `json:"origin_ref_no" column:"damage_report_nodes.origin_ref_no"`
	EventID              *string    `json:"event_id" column:"damage_report_nodes.event_id"`
	CreatedAt            time.Time  `json:"created_at" column:"damage_report_nodes.created_at"`
}
