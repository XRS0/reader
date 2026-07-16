function channel(value: number): number {
  const normalized = value / 255
  return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4
}

export function contrastRatio(first: string, second: string): number {
  const parse = (color: string) => {
    const match = /^#([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(color)
    if (!match) return [0, 0, 0]
    return [
      Number.parseInt(match[1]!, 16),
      Number.parseInt(match[2]!, 16),
      Number.parseInt(match[3]!, 16)
    ]
  }
  const luminance = (color: string) => {
    const [red, green, blue] = parse(color)
    return 0.2126 * channel(red!) + 0.7152 * channel(green!) + 0.0722 * channel(blue!)
  }
  const one = luminance(first)
  const two = luminance(second)
  return (Math.max(one, two) + 0.05) / (Math.min(one, two) + 0.05)
}
