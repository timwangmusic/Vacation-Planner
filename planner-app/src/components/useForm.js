import {useState} from 'react'
const useForm = (callback) => {
    const [values, setValues] = useState({})

    const handleSubmit = (event) => {
        if (event) {
            event.preventDefault()
        var proxyUrl = 'https://cors-anywhere.herokuapp.com/',
        targetUrl = 'https://best-vacation-planner.herokuapp.com/login',
        testUrl = 'http://www.seekerify.com/'

        fetch ( proxyUrl+targetUrl, {
            method: 'POST',
            headers: {
		        'content-Type': 'application/json',
		        'Accept' : 'application/json',
		    },
            body: JSON.stringify(values)
        })
        .then((response) => {
            return response.json()
            })
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
