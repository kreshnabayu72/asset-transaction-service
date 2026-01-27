package change_log

type GetChangeLogListWorkflowParam struct {
	ClientId string
	// Query    dto.GetChangeLogQuery
}

// func GetChangeLogListWorkflow(ctx workflow.Context, param GetChangeLogListWorkflowParam) (*dto.PaginatedChangeLogResult, error) {
// ao := workflow.ActivityOptions{
// 	StartToCloseTimeout: 1 * time.Minute,
// 	RetryPolicy: &temporal.RetryPolicy{
// 		MaximumAttempts: 1,
// 	},
// }

// ctx = workflow.WithActivityOptions(ctx, ao)

// // 2. Get paginated changelogs
// var changeLogs dto.PaginatedChangeLogResult
// err := workflow.ExecuteActivity(ctx, "GetChangeLogListActivity", user_activity.GetChangeLogListActivityParam{
// 	ClientId: param.ClientId,
// 	Query:    param.Query,
// }).Get(ctx, &changeLogs)
// if err != nil {
// 	return nil, err
// }

// return &changeLogs, nil
// }
