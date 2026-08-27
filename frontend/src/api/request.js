import axios from 'axios'
const request=axios.create({baseURL:'/api',timeout:30000})
request.interceptors.response.use(r=>r.data,e=>Promise.reject(new Error(e.response?.data?.error||e.response?.data?.message||e.message)))
export default request
