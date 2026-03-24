<template>
  <LMap
    style="height: 650px"
    :zoom="6"
    :center="[47.21322, -1.559482]"
    :use-global-leaflet="false"
  >
    <LTileLayer
      url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      attribution="&amp;copy; <a href=&quot;https://www.openstreetmap.org/&quot;>OpenStreetMap</a> contributors"
      layer-type="base"
      name="OpenStreetMap"
    />
  </LMap>
</template>