export const start = new Date(Date.now() + 48 * 3600_000);
start.setUTCHours(8, 0, 0, 0);
export const fixtureTime = minutes => new Date(start.getTime() + minutes * 60_000).toISOString();
export const trackingId = 'appt-demo-1';
export const trackingToken = 'hash-test-123';

export function bookingFixture(req, res, empty = false) {
  const path = req.url.split('?')[0];
  const send = (data, status = 200) => {
    res.writeHead(status, { 'Content-Type': 'application/json' }).end(JSON.stringify(data));
    return true;
  };
  if (path === '/v1/public/therapists') return send(empty ? [] : [{
    id: 'fixture-therapist', name: 'دکتر مریم علوی', slug: 'fixture-therapist',
    title: 'درمانگر', session_duration_minutes: 45,
  }]);
  if (path.startsWith('/v1/public/therapists/') && path.endsWith('/slots')) return send(empty ? [] : [{
    id: 'fixture-slot', therapist_id: 'fixture-therapist', starts_at: fixtureTime(0),
    ends_at: fixtureTime(45), status: 'available',
  }]);
  if (path === '/v1/public/appointments/' + trackingId) {
    if (empty || ![trackingToken, 'test-token-hash'].includes(req.headers['x-guest-lookup-hash'])) return send({}, 404);
    return send({
      id: trackingId, guest_name: 'مراجع آزمایشی', therapist_id: 'fixture-therapist', slot_id: 'fixture-slot',
      starts_at: fixtureTime(0), ends_at: fixtureTime(45), status: 'pending',
    });
  }
  if (path === '/v1/public/appointments/' + trackingId + '/cancel' && req.method === 'POST') {
    if (empty || req.headers['x-guest-lookup-hash'] !== trackingToken) return send({}, 404);
    return send({ id: trackingId, status: 'cancelled' });
  }
  if (path === '/v1/public/appointments/unavailable') return send({}, 503);
  return false;
}
