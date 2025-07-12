package context

//// GetUserFromEcho 从 Echo context 中获取当前用户
//func GetUserFromEcho(c echo.Context) (*ent.User, bool) {
//	return GetUserFromContext(c.Request().Context())
//}
//
//// GetRequestIDFromEcho 从 Echo context 中获取 request ID
//func GetRequestIDFromEcho(c echo.Context) (string, bool) {
//	return GetRequestIDFromContext(c.Request().Context())
//}
//
//// SetUserToEcho 将用户信息存储到Echo context中
//func SetUserToEcho(c echo.Context, user *ent.User) {
//	ctx := c.Request().Context()
//	userCtx := WithUser(ctx, user)
//	c.SetRequest(c.Request().WithContext(userCtx))
//}
//
//// SetRequestIDToEcho 将request ID存储到Echo context中
//func SetRequestIDToEcho(c echo.Context, requestID string) {
//	ctx := c.Request().Context()
//	requestIDCtx := WithRequestID(ctx, requestID)
//	c.SetRequest(c.Request().WithContext(requestIDCtx))
//}
