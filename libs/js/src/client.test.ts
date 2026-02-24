
import { GaiaClient } from './client';
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import * as path from 'path';

// Mock grpc
jest.mock('@grpc/grpc-js');
jest.mock('@grpc/proto-loader');

describe('GaiaClient', () => {
    let client: GaiaClient;
    let mockGrpcClient: any;

    beforeEach(async () => {
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

        mockGrpcClient = {
            waitForReady: jest.fn((deadline, callback) => callback(null)),
            GetSecret: jest.fn(),
            ListSecrets: jest.fn(),
            close: jest.fn(),
        };

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
        const mockResponse = { id: 'test-id', value: 'test-value' };
        mockGrpcClient.GetSecret.mockImplementation((req: any, callback: any) => {
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
        const mockResponse = {
            namespaces: [
                {
                    name: 'ns1',
                    secrets: [{ id: 'k1', value: 'v1' }]
                }
            ]
        };
        mockGrpcClient.ListSecrets.mockImplementation((req: any, callback: any) => {
            callback(null, mockResponse);
        });

        const result = await client.listSecrets('ns1');
        expect(result).toEqual({ ns1: { k1: 'v1' } });
    });

    test('loadEnv default behavior (key only)', async () => {
        const mockResponse = {
            namespaces: [
                {
                    name: 'ns-one',
                    secrets: [{ id: 'key-one', value: 'val1' }]
                }
            ]
        };
        mockGrpcClient.ListSecrets.mockImplementation((req: any, callback: any) => {
            callback(null, mockResponse);
        });

        await client.loadEnv();
        
        expect(process.env.KEY_ONE).toBe('val1');
        delete process.env.KEY_ONE;
    });

    test('loadEnv with prefix and namespace configured', async () => {
        const mockResponse = {
            namespaces: [
                {
                    name: 'ns-one',
                    secrets: [{ id: 'key-one', value: 'val1' }]
                }
            ]
        };
        mockGrpcClient.ListSecrets.mockImplementation((req: any, callback: any) => {
            callback(null, mockResponse);
        });

        await client.loadEnv({ prefix: 'GAIA', useNamespace: true });
        
        expect(process.env.GAIA_NS_ONE_KEY_ONE).toBe('val1');
        delete process.env.GAIA_NS_ONE_KEY_ONE;
    });
});
