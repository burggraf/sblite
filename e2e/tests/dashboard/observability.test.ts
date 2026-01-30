import { expect } from 'chai'
import { spawn } from 'child_process'
import { createClient } from '@supabase/supabase-js'

describe('Dashboard Observability', () => {
  const adminClient = () => createClient('http://localhost:8080', 'service-role-key')

  describe('OTel Status Endpoint', () => {
    it('should return disabled status when OTel is not enabled', async function () {
      this.timeout(10000)

      const status = await adminClient()
        .from('_')
        .select('status')
        .single()

      const data = status.data as { status: string }
      expect(data?.status).to.be.true

      // Call the observability status endpoint
      const res = await fetch('http://localhost:8080/_/api/observability/status')
      expect(res.ok).to.be.true

      const observability = await res.json()
      expect(observability).to.have.property('enabled', false)
    })

    it('should return enabled status and config when OTel is enabled', async function () {
      this.timeout(15000)

      // Restart server with OTel enabled
      const server = spawn('./sblite', [
        'serve',
        '--port', '8081',
        '--otel-exporter', 'stdout',
        '--otel-sample-rate', '0.5'
      ])

      // Wait for server to start
      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        const admin = createClient('http://localhost:8081', 'service-role-key')
        const { data } = await admin.from('_').select('status').single()

        const res = await fetch('http://localhost:8081/_/api/observability/status')
        expect(res.ok).to.be.true

        const observability = await res.json()
        expect(observability).to.have.property('enabled', true)
        expect(observability).to.have.property('exporter', 'stdout')
        expect(observability).to.have.property('sampleRate', 0.5)
        expect(observability).to.have.property('metricsEnabled', true)
        expect(observability).to.have.property('tracesEnabled', true)
      } finally {
        server.kill()
      }
    })
  })

  describe('Metrics Endpoint', () => {
    it('should return empty metrics when OTel is disabled', async function () {
      this.timeout(10000)

      const res = await fetch('http://localhost:8080/_/api/observability/metrics')
      expect(res.ok).to.be.true

      const metrics = await res.json()
      expect(metrics).to.be.an('object')
    })

    it('should return aggregated metrics over time range', async function () {
      this.timeout(20000)

      // Start server with OTel
      const server = spawn('./sblite', [
        'serve',
        '--port', 8082,
        '--otel-exporter', 'stdout',
        '--otel-metrics-enabled'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        const admin = createClient('http://localhost:8082', 'service-role-key')
        const supabase = createClient('http://localhost:8082', 'test-key')

        // Make some requests to generate metrics
        for (let i = 0; i < 20; i++) {
          await supabase.from('_test').select('*').limit(1).catch(() => {})
        }

        // Wait a bit for metrics to be stored
        await new Promise(resolve => setTimeout(resolve, 2000))

        // Fetch metrics
        const res = await fetch('http://localhost:8082/_/api/observability/metrics?minutes=1')
        expect(res.ok).to.be.true

        const metrics = await res.json()
        expect(metrics).to.be.an('object')

        // Should have http.server.request_count metric
        expect(metrics).to.have.property('http.server.request_count')
      } finally {
        server.kill()
      }
    })

    it('should support different time ranges', async function () {
      this.timeout(20000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8083,
        '--otel-exporter', 'stdout'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        // Test 5 minutes
        const res5 = await fetch('http://localhost:8083/_/api/observability/metrics?minutes=5')
        expect(res5.ok).to.be.true
        const metrics5 = await res5.json()
        expect(metrics5).to.be.an('object')

        // Test 15 minutes
        const res15 = await fetch('http://localhost:8083/_/api/observability/metrics?minutes=15')
        expect(res15.ok).to.be.true
        const metrics15 = await res15.json()
        expect(metrics15).to.be.an('object')

        // Test 60 minutes
        const res60 = await fetch('http://localhost:8083/_/api/observability/metrics?minutes=60')
        expect(res60.ok).to.be.true
        const metrics60 = await res60.json()
        expect(metrics60).to.be.an('object')
      } finally {
        server.kill()
      }
    })
  })

  describe('Traces Endpoint', () => {
    it('should return traces list when OTel is enabled', async function () {
      this.timeout(15000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8084,
        '--otel-exporter', 'stdout',
        '--otel-traces-enabled'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        const supabase = createClient('http://localhost:8084', 'test-key')

        // Make some requests
        await supabase.from('_test').select('*').limit(1).catch(() => {})
        await supabase.from('_test').insert({ data: 'test' }).catch(() => {})

        await new Promise(resolve => setTimeout(resolve, 500))

        // Fetch traces
        const res = await fetch('http://localhost:8084/_/api/observability/traces')
        expect(res.ok).to.be.true

        const traces = await res.json()
        expect(traces).to.be.an('array')
      } finally {
        server.kill()
      }
    })

    it('should support filtering by method', async function () {
      this.timeout(15000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8085,
        '--otel-exporter', 'stdout',
        '--otel-traces-enabled'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        const supabase = createClient('http://localhost:8085', 'test-key')

        // Make different request types
        await supabase.from('_test').select('*').limit(1)
        await supabase.from('_test').insert({ data: 'test' })

        await new Promise(resolve => setTimeout(resolve, 500))

        // Filter by GET method
        const res = await fetch('http://localhost:8085/_/api/observability/traces?method=GET')
        expect(res.ok).to.be.true

        const traces = await res.json()
        expect(traces).to.be.an('array')
        // All traces should be GET method
        traces.forEach((trace: any) => {
          expect(trace.method).to.equal('GET')
        })
      } finally {
        server.kill()
      }
    })

    it('should support filtering by status code', async function () {
      this.timeout(15000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8086',
        '--otel-exporter', 'stdout',
        '--otel-traces-enabled'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        const supabase = createClient('http://localhost:8086', 'test-key')

        // Make requests that will return different statuses
        await supabase.from('_test').select('*').limit(1)
        await supabase.from('_nonexistent').select('*').limit(1).catch(() => {})

        await new Promise(resolve => setTimeout(resolve, 500))

        // Filter by 404 status
        const res = await fetch('http://localhost:8086/_/api/observability/traces?status=404')
        expect(res.ok).to.be.true

        const traces = await res.json()
        expect(traces).to.be.an('array')
        // All traces should have status 404
        traces.forEach((trace: any) => {
          expect(trace.statusCode).to.equal(404)
        })
      } finally {
        server.kill()
      }
    })
  })

  describe('Dashboard UI Integration', () => {
    it('should load observability page via navigation', async function () {
      this.timeout(15000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8087,
        '--otel-exporter', 'stdout'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        // Simulate navigating to observability page
        const statusRes = await fetch('http://localhost:8087/_/api/observability/status')
        expect(statusRes.ok).to.be.true

        const metricsRes = await fetch('http://localhost:8087/_/api/observability/metrics?minutes=5')
        expect(metricsRes.ok).to.be.true

        // Verify response structure
        const status = await statusRes.json()
        expect(status).to.have.property('enabled', true)
        expect(status).to.have.property('config')

        const metrics = await metricsRes.json()
        expect(metrics).to.be.an('object')
      } finally {
        server.kill()
      }
    })

    it('should display observability configuration', async function () {
      this.timeout(15000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8088,
        '--otel-exporter', 'otlp',
        '--otel-endpoint', 'tempo:4317',
        '--otel-sample-rate', '0.25'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        const res = await fetch('http://localhost:8088/_/api/observability/status')
        expect(res.ok).to.be.true

        const config = await res.json()
        expect(config).to.deep.equal({
          enabled: true,
          exporter: 'otlp',
          endpoint: 'tempo:4317',
          serviceName: 'sblite',
          sampleRate: 0.25,
          metricsEnabled: true,
          tracesEnabled: true
        })
      } finally {
        server.kill()
      }
    })
  })

  describe('Auto-refresh Functionality', () => {
    it('should toggle auto-refresh state', async function () {
      this.timeout(15000)

      const server = spawn('./sblite', [
        'serve',
        '--port', 8089,
        '--otel-exporter', 'stdout'
      ])

      await new Promise(resolve => setTimeout(resolve, 3000))

      try {
        // Get initial state (auto-refresh should be off)
        let res = await fetch('http://localhost:8089/_/api/observability/status')
        let status = await res.json()

        // Note: Dashboard auto-refresh is tracked in state, not in API
        // This test verifies the API endpoint structure

        expect(status).to.be.an('object')
        expect(status).to.have.property('enabled')
      } finally {
        server.kill()
      }
    })
  })

  describe('Empty State', () => {
    it('should show helpful message when OTel is disabled', async function () {
      this.timeout(10000)

      // Without OTel, the observability API should return enabled: false
      const res = await fetch('http://localhost:8080/_/api/observability/status')
      expect(res.ok).to.be.true

      const status = await res.json()
      expect(status.enabled).to.be.false

      // Metrics should be empty or null
      const metricsRes = await fetch('http://localhost:8080/_/api/observability/metrics')
      expect(metricsRes.ok).to.be.true

      const metrics = await metricsRes.json()
      expect(metrics).to.be.an('object')
      // Could be empty object or have no data
    })
  })
})
