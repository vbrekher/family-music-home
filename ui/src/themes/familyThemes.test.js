import { describe, it, expect } from 'vitest'
import themes from './index'
import { getContrastRatio, createTheme } from '@material-ui/core/styles'
describe('Preserved family themes', () => {
  it('keeps every built-in choice and six requested themes', () => {
    expect(Object.keys(themes)).toHaveLength(27)
    for (const name of [
      'SpotifyTheme',
      'CatppuccinMochaTheme',
      'TokyoNightTheme',
      'NordTheme',
      'DraculaTheme',
      'RosePineMoonTheme',
    ]) {
      expect(themes[name]).toBeTruthy()
      const palette = createTheme(themes[name]).palette
      {
        const ratio = getContrastRatio(
          palette.text.primary,
          palette.background.default,
        )
        console.info(name, 'body contrast', ratio)
        expect(ratio).toBeGreaterThanOrEqual(4.5)
        const label = themes[name].overrides?.MuiFormLabel?.root?.color
        if (label)
          expect(
            getContrastRatio(label, palette.background.paper),
          ).toBeGreaterThanOrEqual(4.5)
      }
    }
  })
})
