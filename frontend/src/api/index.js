import request from './request'
export const alertsApi={list:()=>request.get('/alerts')}
export const clusterApi={summary:()=>request.get('/cluster/summary'),pods:()=>request.get('/cluster/pods'),deployments:()=>request.get('/cluster/deployments'),events:(limit=100)=>request.get('/cluster/events',{params:{limit}}),logs:(pod,params={})=>request.get(`/cluster/pods/${encodeURIComponent(pod)}/logs`,{params})}
export const knowledgeApi={list:()=>request.get('/knowledge/documents'),upload:file=>{const data=new FormData();data.append('file',file);return request.post('/upload',data)},remove:name=>request.delete(`/knowledge/documents/${encodeURIComponent(name)}`)}
export { request }
