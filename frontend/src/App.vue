<template><n-config-provider :theme-overrides="theme"><n-message-provider><n-notification-provider><router-view /></n-notification-provider></n-message-provider></n-config-provider></template>
<script setup>const theme={common:{primaryColor:'#18a058',primaryColorHover:'#36ad6a',primaryColorPressed:'#0c7a43',borderRadius:'8px'}}</script>
