package plugin

// grpc_client.go documents the design decision for the daemon ↔ plugin gRPC path.
//
// The daemon calls Manager.Get(vaultType) which returns an sdk.Provider backed
// by sdk.VaultGRPCClient. No additional wrapper is needed here: the sdk package
// already provides the client-side adapter (sdk.VaultGRPCClient) that implements
// sdk.Provider and delegates calls to vaultv1.VaultProviderClient.
