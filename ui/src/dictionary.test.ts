import dictionary, { getResourceParams } from './dictionary';

test('test getResourceParams', () => {
    const httpcliParams = getResourceParams(dictionary, 'httpclient');

    expect(httpcliParams.length).toBe(3);

    const postgresParams = getResourceParams(dictionary, 'postgres');

    expect(postgresParams.length).toBe(2);
});
