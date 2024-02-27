import { check } from 'k6';
import http from 'k6/http';

export const options = {
  scenarios: {
        constant_request_rate: {
            executor: 'constant-arrival-rate',
            rate: 3000, // Number of iterations to start during each timeUnit period.
            timeUnit: '1s', // Period of time to apply the rate value.
            maxVUs: 200,
            preAllocatedVUs: 100,
            duration: '2m',
        },
    },
    thresholds: {
        'http_req_failed{scenario:constant_request_rate}': ['rate<0.01'],
        'http_req_duration{scenario:constant_request_rate}': ['p(95)<20000'],
    }, };


export default function () {
    let url = 'http://34.102.197.22/client/env/123/target/foobar/evaluations';
    let res = http.get(url);
 check(res, {
    'is status 200': (r) => r.status === 200,
  });
}
