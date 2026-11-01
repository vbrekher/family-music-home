import { useEffect, useState } from 'react'
export function readChildPolicy() {
  try {
    return JSON.parse(localStorage.getItem('family-child') || 'null')
  } catch (_) {
    return null
  }
}
export function setChildPolicy(policy) {
  if (policy) localStorage.setItem('family-child', JSON.stringify(policy))
  else localStorage.removeItem('family-child')
  window.dispatchEvent(new Event('family-policy'))
}
export function useChildPolicy() {
  const [policy, set] = useState(readChildPolicy)
  useEffect(() => {
    const update = () => set(readChildPolicy())
    window.addEventListener('family-policy', update)
    return () => window.removeEventListener('family-policy', update)
  }, [])
  return policy
}
