package auth

import "context"

type contextKey string

const deviceIDKey contextKey = "device_id"

// ContextWithDeviceID injects the device ID into the context.
func ContextWithDeviceID(ctx context.Context, deviceID string) context.Context {
	return context.WithValue(ctx, deviceIDKey, deviceID)
}

// DeviceIDFromContext retrieves the authenticated device ID from the context.
func DeviceIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(deviceIDKey).(string)
	return id, ok
}
