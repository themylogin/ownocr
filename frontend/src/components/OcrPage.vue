<script setup lang="ts">
import { computed } from 'vue'
import { useThemeVars } from 'naive-ui'
import type { PageOcr } from '../api/client'

const props = defineProps<{
  src: string
  width: number
  height: number
  scale: number
  text: PageOcr | undefined
}>()

const themeVars = useThemeVars()

// The share of the box height the glyphs take, and where the baseline sits in
// it. Both are read off the box, so a line stays inside the box the engine
// reported for it.
const glyphHeight = 0.78
const baseline = 0.82

const size = computed(() => ({
  width: `${Math.floor(props.width * props.scale)}px`,
  height: `${Math.floor(props.height * props.scale)}px`,
}))

// The strip the page carries of the page beside it, as a rectangle in the space
// the boxes count in. The server has already dropped the lines inside it, so
// this covers ink and never a glyph.
const trim = computed(() => {
  const found = props.text?.trim
  if (!found) {
    return null
  }
  return {
    x: found.side === 'left' ? 0 : props.width - found.column,
    width: found.column,
  }
})

const lines = computed(() =>
  (props.text?.lines ?? [])
    .filter((line) => line.text !== '' && line.box[2] > line.box[0] && line.box[3] > line.box[1])
    .map((line, index) => {
      const [left, top, right, bottom] = line.box
      return {
        key: index,
        text: line.text,
        fromLineReading: line.fromLineReading === true,
        x: left,
        y: top + (bottom - top) * baseline,
        top,
        length: right - left,
        height: bottom - top,
        fontSize: (bottom - top) * glyphHeight,
      }
    }),
)

function subpath(line: { x: number; top: number; length: number; height: number }) {
  return `M${line.x} ${line.top}H${line.x + line.length}V${line.top + line.height}H${line.x}Z`
}

// Two paths and not one: a line the merge filled from the line engine stands on a
// grey ground, and the rest on white. The text of such a line is the page as much
// as any other, so it is drawn in black like the rest and the ground says where it
// came from.
const grounds = computed(() =>
  lines.value
    .filter((line) => !line.fromLineReading)
    .map(subpath)
    .join(''),
)

const substitutedGrounds = computed(() =>
  lines.value
    .filter((line) => line.fromLineReading)
    .map(subpath)
    .join(''),
)
</script>

<template>
  <div class="page" :style="size" data-testid="ocr-overlay">
    <img :src="src" alt="" :width="width" :height="height" loading="lazy" />
    <div class="veil"></div>

    <svg
      v-if="lines.length || trim"
      class="text"
      :viewBox="`0 0 ${width} ${height}`"
      preserveAspectRatio="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <!-- The strip of the page beside this one, covered before anything is
           drawn over it. The veil alone leaves it showing through. -->
      <rect
        v-if="trim"
        class="trim"
        :x="trim.x"
        y="0"
        :width="trim.width"
        :height="height"
        data-testid="ocr-trim"
      />

      <!-- Every ground first, then every line. SVG paints in document order, so
           a ground drawn beside its own line would cover the glyphs of the line
           above that reach into its box.

           One path holds every ground of the page, one subpath per line. A book
           of 1000 pages draws 1000 shapes here instead of 45,000, and the
           measurements in the change `add-full-book-find` name that count, not
           the text, as what the reader could not carry. -->
      <g class="grounds">
        <path :d="grounds" />
        <path
          v-if="substitutedGrounds"
          class="substituted"
          :d="substitutedGrounds"
          data-testid="ocr-substituted-grounds"
        />
      </g>

      <g class="glyphs">
        <text
          v-for="line in lines"
          :key="line.key"
          :x="line.x"
          :y="line.y"
          :textLength="line.length"
          :font-size="line.fontSize"
          lengthAdjust="spacingAndGlyphs"
          :class="{ substituted: line.fromLineReading }"
          data-testid="ocr-line"
        >
          {{ line.text }}
        </text>
      </g>
    </svg>
  </div>
</template>

<style scoped>
.page {
  position: relative;
  background: v-bind('themeVars.cardColor');
}

img,
.veil,
.text {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  display: block;
}

/* fill, not contain: this box is the space the boxes are placed in, so the
   image must cover all of it. Letterboxing it would leave the text drawn over
   the gap instead of over the line. */
img {
  object-fit: fill;
}

.veil {
  background: #fff;
  opacity: 0.5;
}

/* The viewBox carries the scale, so the boxes and the glyphs follow the image
   with no arithmetic of their own. */
.text {
  fill: #000;
  font-family: serif;
}

/* Each line stands on its own white ground, so the print under it reaches no
   glyph. The veil alone leaves the two readings on top of one another. */
.grounds path {
  fill: #fff;
}

/* A line the merge filled from the line engine, because the reading engine did not
   read it alike. It is the page as much as any other line, so its text stays black
   and the ground under it says where the text came from. On a page whose body came
   from the reading engine and whose footnotes did not, nothing else would say so.

   Grey and light: dark enough to see against the white of the rest, light enough to
   read black text on. */
.grounds path.substituted {
  fill: #e0e0e0;
}

/* Opaque, unlike the veil: this covers print that belongs to another page. */
.trim {
  fill: #fff;
}
</style>
