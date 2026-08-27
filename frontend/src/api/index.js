import request from './request'
export const alertsApi={list:()=>request.get('/alerts')}
export const clusterApi={summary:()=>request.get('/cluster/summary'),pods:()=>request.get('/cluster/pods'),deployments:()=>request.get('/cluster/deployments'),events:(limit=100)=>request.get('/cluster/events',{params:{limit}}),logs:(pod,params={})=>request.get(`/cluster/pods/${encodeURIComponent(pod)}/logs`,{params})}
export const maintenanceDocumentApi={list:(params={})=>request.get('/maintenance-documents',{params}),get:name=>request.get(`/maintenance-documents/${encodeURIComponent(name)}`),upload:file=>{const data=new FormData();data.append('file',file);return request.post('/maintenance-documents',data)},remove:name=>request.delete(`/maintenance-documents/${encodeURIComponent(name)}`)}
export const knowledgeApi=maintenanceDocumentApi
export const backgroundApi={list:()=>request.get('/background/tasks'),get:id=>request.get(`/background/tasks/${id}`),approve:id=>request.post(`/background/tasks/${id}/approve`,{}, {headers:{'X-Operator':'frontend-user'}}),reject:id=>request.post(`/background/tasks/${id}/reject`,{}, {headers:{'X-Operator':'frontend-user'}})}
export const incidentApi={list:()=>request.get('/incidents'),get:id=>request.get(`/incidents/${id}`),create:data=>request.post('/incidents',data)}
export const workOrderApi={list:()=>request.get('/work-orders'),create:data=>request.post('/work-orders',data)}
export { request }
