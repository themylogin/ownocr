import { ref } from 'vue'

// The view asks for the whole window instead of the content column. Only the
// OCR reader does, because it shows two pages side by side.
const wide = ref(false)

export function useLayout() {
  return { wide }
}
