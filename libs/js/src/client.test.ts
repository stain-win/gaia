import { GaiaClient } from './client';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';

// Mock grpc
jest.mock('@grpc/grpc-js');
jest.mock('@grpc/proto-loader');

type GetSecretResponse = { id: string; value: string };
type ListSecretsResponse = {
    namespaces: { name: string; secrets: { id: string; value: string }[] }[];
};

type UnaryCallback<T> = (error: grpc.ServiceError | null, response: T) => void;

type MockGrpcClient = {
    waitForReady: jest.Mock<void, [number, (error?: Error | null) => void]>;
    GetSecret: jest.Mock<void, [unknown, UnaryCallback<GetSecretResponse>]>;
    ListSecrets: jest.Mock<void, [unknown, UnaryCallback<ListSecretsResponse>]>;
    close: jest.Mock<void, []>;
};

describe('GaiaClient', () => {
    let client: GaiaClient;
    let mockGrpcClient: MockGrpcClient;

    beforeEach(async () => {
        mockGrpcClient = {
            waitForReady: jest.fn((deadline: number, callback: (error?: Error | null) => void) => callback(null)),
            GetSecret: jest.fn(),
            ListSecrets: jest.fn(),
            close: jest.fn(),
        };

        // Mock loadPackageDefinition
        (grpc.loadPackageDefinition as jest.Mock).mockReturnValue({
            gaia: {
                GaiaClient: jest.fn().mockImplementation(() => {
                    return mockGrpcClient;
                })
            }
        });

        // Mock protoLoader.load
        (protoLoader.load as jest.Mock).mockResolvedValue({});

        client = new GaiaClient({
            address: 'localhost:50051',
            insecure: true
        });
        await client.connect();
    });

    afterEach(async () => {
        await client.close();
    });

    test('getSecret success', async () => {
        const mockResponse: GetSecretResponse = { id: 'test-id', value: 'test-value' };
        mockGrpcClient.GetSecret.mockImplementation((_req: unknown, callback: UnaryCallback<GetSecretResponse>) => {
            callback(null, mockResponse);
        });

        const result = await client.getSecret('test-ns', 'test-id');
        expect(result).toBe('test-value');
        expect(mockGrpcClient.GetSecret).toHaveBeenCalledWith(
            { namespace: 'test-ns', id: 'test-id' },
            expect.any(Function)
        );
    });

    test('listSecrets success', async () => {
        const mockResponse: ListSecretsResponse = {
            namespaces: [
                {
                    name: 'ns1',
                    secrets: [{ id: 'k1', value: 'v1' }]
                }
            ]
        };
        mockGrpcClient.ListSecrets.mockImplementation((_req: unknown, callback: UnaryCallback<ListSecretsResponse>) => {
            callback(null, mockResponse);
        });

        const result = await client.listSecrets('ns1');
        expect(result).toEqual({ ns1: { k1: 'v1' } });
    });

    test('loadEnv default behavior (key only)', async () => {
        const mockResponse: ListSecretsResponse = {
            namespaces: [
                {
                    name: 'ns-one',
                    secrets: [{ id: 'key-one', value: 'val1' }]
                }
            ]
        };
        mockGrpcClient.ListSecrets.mockImplementation((_req: unknown, callback: UnaryCallback<ListSecretsResponse>) => {
            callback(null, mockResponse);
        });

        await client.loadEnv();
        
        expect(process.env.KEY_ONE).toBe('val1');
        delete process.env.KEY_ONE;
    });

    test('loadEnv with prefix and namespace configured', async () => {
        const mockResponse: ListSecretsResponse = {
            namespaces: [
                {
                    name: 'ns-one',
                    secrets: [{ id: 'key-one', value: 'val1' }]
                }
            ]
        };
        mockGrpcClient.ListSecrets.mockImplementation((_req: unknown, callback: UnaryCallback<ListSecretsResponse>) => {
            callback(null, mockResponse);
        });

        await client.loadEnv({ prefix: 'GAIA', useNamespace: true });
        
        expect(process.env.GAIA_NS_ONE_KEY_ONE).toBe('val1');
        delete process.env.GAIA_NS_ONE_KEY_ONE;
    });
});
