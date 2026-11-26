package query_builder

import "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"

type CheckAvailabilityRow struct {
	Available bool `json:"available" column:"available"`
}

func BuildCheckAvailabilityQuery(name string, excludeID *int64) (string, []any, error) {
	builder := sql_query.NewSQLSelectBuilder[CheckAvailabilityRow]("service_centers").
		SelectRaw("COUNT(*) = 0 AS available").
		Where(sql_query.WhereQuery{
			"": {
				Operator:  sql_query.SQLOperatorRaw,
				Value:     "name_key = LOWER(BTRIM(?))",
				ExtraArgs: []any{name},
			},
		})

	if excludeID != nil {
		builder.Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorNotEqual, Value: *excludeID},
		})
	}

	return builder.Build()
}
