import { describe, expect, it } from 'vitest'
import { closeRing, parseRing } from './validation'

const square = '[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]'

describe('parseRing', () => {
  it('accepts a closed polygon', () => {
    const result = parseRing(square)
    expect(result.ok).toBe(true)
  })

  it.each([
    ['', 'Enter coordinates'],
    ['{bad', 'not valid JSON'],
    ['{"a":1}', 'list of [lat, lng] pairs'],
    ['[[1,2],[3]]', 'Point 2 must be'],
    ['[[95,0],[1,1],[2,2],[95,0]]', 'latitude must be between'],
    ['[[0,190],[1,1],[2,2],[0,190]]', 'longitude must be between'],
    ['[[0,0],[1,1],[0,0]]', 'at least 4 points'],
    ['[[0,0],[1,0],[1,1],[0,1]]', 'repeat the first point'],
    ['[[0,0],[1,1],[0,0],[1,1],[0,0]]', '3 different corners'],
  ])('rejects %s', (input, message) => {
    const result = parseRing(input)
    expect(result.ok).toBe(false)
    if (!result.ok) expect(result.error).toContain(message)
  })
})

describe('closeRing', () => {
  it('appends the first point when missing', () => {
    expect(closeRing([[0, 0], [1, 0], [1, 1]])).toEqual([[0, 0], [1, 0], [1, 1], [0, 0]])
  })
  it('leaves a closed ring alone', () => {
    const ring: [number, number][] = [[0, 0], [1, 0], [1, 1], [0, 0]]
    expect(closeRing(ring)).toBe(ring)
  })
})
