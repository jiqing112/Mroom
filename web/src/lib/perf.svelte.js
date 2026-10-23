/**
 * Rendering budget.
 *
 * "full"   — aurora canvas, glass blur, blurred artwork, grain, motion.
 * "smooth" — flat gradients, opaque surfaces, no blur filters, no looping
 *            animation. Everything that costs GPU per frame is dropped so a
 *            low-end machine can keep decoding audio without stutter.
 *
 * "auto" resolves once at boot from device hints and never changes afterwards.
 */

const KEY = 'mrome.perf.v1'

export const perf = $state({
  mode: 'auto', // 'auto' | 'full' | 'smooth' | 'static'
  smooth: false, // resolved value (static also uses the smooth palette)
  static: false, // extreme low-power: no loop animations at all
  autoSmooth: false, // what auto detection wanted
  reasons: []
})

function detect() {
  const reasons = []
  if (typeof window === 'undefined') return reasons
  if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
    reasons.push('系统开启了「减少动态效果」')
  }
  const cores = navigator.hardwareConcurrency || 8
  if (cores <= 4) reasons.push(`CPU 仅 ${cores} 线程`)
  const mem = navigator.deviceMemory
  if (mem && mem <= 4) reasons.push(`内存 ${mem} GB`)
  if (navigator.connection?.saveData) reasons.push('开启了省流量模式')
  return reasons
}

export function applyPerf() {
  if (typeof document === 'undefined') return
  const reasons = detect()
  perf.reasons = reasons
  perf.autoSmooth = reasons.length > 0
  perf.static = perf.mode === 'static'
  perf.smooth = perf.static
    ? true
    : perf.mode === 'smooth'
      ? true
      : perf.mode === 'full'
        ? false
        : perf.autoSmooth
  const root = document.documentElement
  root.dataset.perf = perf.smooth ? 'smooth' : 'full'
  if (perf.static) root.setAttribute('data-static', 'true')
  else root.removeAttribute('data-static')
}

export function setPerfMode(mode) {
  perf.mode = mode
  try {
    localStorage.setItem(KEY, mode)
  } catch {
    /* private mode */
  }
  applyPerf()
}

export function initPerf() {
  try {
    const saved = localStorage.getItem(KEY)
    if (saved === 'full' || saved === 'smooth' || saved === 'auto' || saved === 'static') perf.mode = saved
  } catch {
    /* ignore */
  }
  applyPerf()
}
