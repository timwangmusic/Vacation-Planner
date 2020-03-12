import {useState} from 'react'
const useForm = (callback) => {
    const [values, setValues] = useState({})

    const handleSubmit = (event) => {
        if (event) {
            event.preventDefault()

        fetch ( 'https://best-vacation-planner.herokuapp.com/login', {
            method: 'POST',
            mode:"no-cors",
            headers: {
		        'content-Type': 'application/json',
		        'Accept' : 'application/json',
            'Cache-Control' : 'no-cache',
            'Host' : 'best-vacation-planner.herokuapp.com',
            'Accept-Encoding' : 'gzip, deflate, br',
            'Connection' : 'keep-alive',
            'Access-Control-Allow-Methods' : 'POST',
            'Access-Control-Allow-headers' : 'Content-Type, Authorization',
            'Access-Control-Allow-Origin' : 'http://localhost:3000',
            'Access-Control-Allow-Credentials' : 'true'
		    },
            body: JSON.stringify(values)
        })
        .then(response => {
            return response.text().then(data => {
                console.log(data)
                console.log('Logged In..')
            })
            })
        .then(contents => console.log(contents))
        .then((values) => {
            console.log('Success:')
        })
        .catch((error) => {
            console.error('Error:', error)
    })
        callback()
    }
    }

    const handleChange = (event) => {
        event.persist()
        setValues(values => ({ ...values,
            [event.target.name]: event.target.value })
)}
    return {
        handleChange,
        handleSubmit,
        values
    }
}

export default useForm
