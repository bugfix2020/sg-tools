import { describe, expect, it } from 'vitest'
import { defaultFollowSyncSnapshot, parseRuleText } from './follow-sync.utils'

describe('follow-sync utils', () => {
	it('parses comma-separated rules with trimmed case-insensitive deduplication', () => {
		expect(parseRuleText(' KeyA, space, keya, , Space ')).toEqual(['KeyA', 'space'])
	})

	it('starts with no runtime windows and empty rules', () => {
		expect(defaultFollowSyncSnapshot()).toEqual({
		state: 'idle',
		main: undefined,
		follows: [],
		rules: { include: [], exclude: [] },
		capturedCount: 0,
		lastCode: undefined,
		lastError: undefined,
	})
	})
})
