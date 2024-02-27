import http from 'k6/http';

export const options = {
  scenarios: {
        constant_request_rate: {
            executor: 'constant-arrival-rate',
            rate: 2500, // Number of iterations to start during each timeUnit period.
            timeUnit: '1s', // Period of time to apply the rate value.
            maxVUs: 10000,
            preAllocatedVUs: 1000,
            duration: '30s',
        },
    },
    thresholds: {
        'http_req_failed{scenario:constant_request_rate}': ['rate<0.01'],
        'http_req_duration{scenario:constant_request_rate}': ['p(95)<20000'],
    }, };

const attempt = 2

export default function () {
    let url = 'localhost:7000/client/env/123/target/foo/evaluations';
    let res = http.get(url, { headers: headers });
}